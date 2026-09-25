package process

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
)

func Test_NewPreflight(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockWorker := NewMockPool(ctrl)

	log := slog.New(slog.DiscardHandler)

	preflight := NewPreflight(mockPublisher, mockWorker, log)

	assert.NotNil(t, preflight)
	assert.Equal(t, mockPublisher, preflight.publisher)
	assert.Equal(t, mockWorker, preflight.worker)
	assert.Equal(t, log, preflight.log)
}

func Test_matchProcesses(t *testing.T) {
	tests := []struct {
		name      string
		processes []running
		dirs      map[string]string
		expected  []match
	}{
		{
			name: "no matching process kills nothing",
			processes: []running{
				{pid: 100, dir: "/other/dir", name: "node"},
			},
			dirs:     map[string]string{"api": "/project/api"},
			expected: []match{},
		},
		{
			name: "a matching process is returned",
			processes: []running{
				{pid: 100, dir: "/project/api", name: "node"},
			},
			dirs: map[string]string{"api": "/project/api"},
			expected: []match{
				{service: "api", entry: running{pid: 100, dir: "/project/api", name: "node"}},
			},
		},
		{
			name: "every service directory is matched exactly",
			processes: []running{
				{pid: 100, dir: "/project/api", name: "node"},
				{pid: 150, dir: "/project/api-v2", name: "node"},
				{pid: 200, dir: "/project/web", name: "go"},
				{pid: 300, dir: "/other/dir", name: "vim"},
			},
			dirs: map[string]string{"web": "/project/web", "api": "/project/api"},
			expected: []match{
				{service: "api", entry: running{pid: 100, dir: "/project/api", name: "node"}},
				{service: "web", entry: running{pid: 200, dir: "/project/web", name: "go"}},
			},
		},
		{
			name: "the own process is never matched",
			processes: []running{
				{pid: int32(os.Getpid()), dir: "/project/api", name: "fuku"}, // #nosec G115 -- PID fits in int32
				{pid: 200, dir: "/project/api", name: "node"},
			},
			dirs: map[string]string{"api": "/project/api"},
			expected: []match{
				{service: "api", entry: running{pid: 200, dir: "/project/api", name: "node"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := matchProcesses(tt.processes, tt.dirs)

			assert.Equal(t, tt.expected, matches)
		})
	}
}

func Test_absDirs(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	resolved, err := absDirs(map[string]string{"api": "services/api"})

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"api": filepath.Join(wd, "services/api")}, resolved)
}

func Test_Preflight_Cleanup_NoDirectories(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockWorker := NewMockPool(ctrl)

	log := slog.New(slog.DiscardHandler)

	preflight := NewPreflight(mockPublisher, mockWorker, log)

	err := preflight.Cleanup(t.Context(), map[string]string{})

	require.NoError(t, err)
}

func Test_Preflight_Cleanup_KillsMatchingProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockWorker := NewMockPool(ctrl)
	mockWorker.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorker.EXPECT().Release()

	log := slog.New(slog.DiscardHandler)

	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	cmd := exec.Command("sleep", "60")
	cmd.Dir = resolved
	require.NoError(t, cmd.Start())

	exited := make(chan struct{})
	go func() {
		defer close(exited)

		cmd.Wait()
	}()

	t.Cleanup(func() {
		cmd.Process.Kill()
		<-exited
	})

	killed := func(pid int) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.PreflightKilled)

			return msg.Type == contracts.EventPreflightKilled && ok && data.Service == "api" && data.PID == pid
		})
	}
	completed := func(count int) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.PreflightComplete)

			return msg.Type == contracts.EventPreflightComplete && ok && data.Killed == count
		})
	}

	mockPublisher.EXPECT().Publish(contracts.Message{
		Type: contracts.EventPreflightStarted,
		Data: contracts.PreflightStarted{Services: []string{"api"}},
	}).Return(nil)
	mockPublisher.EXPECT().Publish(killed(cmd.Process.Pid)).Return(nil)
	mockPublisher.EXPECT().Publish(completed(1)).Return(nil)

	preflight := NewPreflight(mockPublisher, mockWorker, log)

	err = preflight.Cleanup(t.Context(), map[string]string{"api": resolved})

	require.NoError(t, err)

	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		t.Fatal("the process was not killed")
	}
}

func Test_Preflight_Cleanup_CancelledContextStopsKills(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockWorker := NewMockPool(ctrl)
	mockWorker.EXPECT().Acquire(gomock.Any()).Return(context.Canceled)

	log := slog.New(slog.DiscardHandler)

	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	cmd := exec.Command("sleep", "60")
	cmd.Dir = resolved
	require.NoError(t, cmd.Start())

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	completed := func(count int) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.PreflightComplete)

			return msg.Type == contracts.EventPreflightComplete && ok && data.Killed == count
		})
	}

	mockPublisher.EXPECT().Publish(contracts.Message{
		Type: contracts.EventPreflightStarted,
		Data: contracts.PreflightStarted{Services: []string{"api"}},
	}).Return(nil)
	mockPublisher.EXPECT().Publish(completed(0)).Return(nil)

	preflight := NewPreflight(mockPublisher, mockWorker, log)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err = preflight.Cleanup(ctx, map[string]string{"api": resolved})

	require.NoError(t, err)
}

func Test_Preflight_Cleanup_NoMatchingProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockWorker := NewMockPool(ctrl)

	log := slog.New(slog.DiscardHandler)

	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	completed := func(count int) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.PreflightComplete)

			return msg.Type == contracts.EventPreflightComplete && ok && data.Killed == count
		})
	}

	mockPublisher.EXPECT().Publish(contracts.Message{
		Type: contracts.EventPreflightStarted,
		Data: contracts.PreflightStarted{Services: []string{"api"}},
	}).Return(nil)
	mockPublisher.EXPECT().Publish(completed(0)).Return(nil)

	preflight := NewPreflight(mockPublisher, mockWorker, log)

	err = preflight.Cleanup(t.Context(), map[string]string{"api": resolved})

	require.NoError(t, err)
}

func Test_Preflight_Cleanup_ScanFails(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("HOST_PROC only redirects the process table on linux")
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockWorker := NewMockPool(ctrl)

	log := slog.New(slog.DiscardHandler)

	t.Setenv("HOST_PROC", filepath.Join(t.TempDir(), "missing"))

	completed := func(count int) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.PreflightComplete)

			return msg.Type == contracts.EventPreflightComplete && ok && data.Killed == count
		})
	}

	mockPublisher.EXPECT().Publish(contracts.Message{
		Type: contracts.EventPreflightStarted,
		Data: contracts.PreflightStarted{Services: []string{"api"}},
	}).Return(nil)
	mockPublisher.EXPECT().Publish(completed(0)).Return(nil)

	preflight := NewPreflight(mockPublisher, mockWorker, log)

	err := preflight.Cleanup(t.Context(), map[string]string{"api": "/project/api"})

	require.ErrorContains(t, err, "failed to scan processes")
}

func Test_scan(t *testing.T) {
	entries, err := scan()

	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	ownPID := int32(os.Getpid()) // #nosec G115 -- PID fits in int32
	found := false

	for _, e := range entries {
		if e.pid == ownPID {
			found = true

			break
		}
	}

	assert.True(t, found)
}

func Test_kill(t *testing.T) {
	tests := []struct {
		name   string
		before func(t *testing.T) int32
	}{
		{
			name: "a process exits on SIGTERM",
			before: func(t *testing.T) int32 {
				cmd := exec.Command("sleep", "60")
				require.NoError(t, cmd.Start())

				go cmd.Wait()

				return int32(cmd.Process.Pid) // #nosec G115 -- PID fits in int32
			},
		},
		{
			name: "a process that does not exist needs nothing",
			before: func(_ *testing.T) int32 {
				return 2147483647
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pid := tt.before(t)

			err := kill(pid)

			require.NoError(t, err)
		})
	}
}

func Test_kill_IgnoresSIGTERM(t *testing.T) {
	cmd := exec.Command("sh", "-c", "trap '' TERM; echo ready; sleep 60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())

	ready := make([]byte, 6)
	_, err = io.ReadFull(stdout, ready)
	require.NoError(t, err)

	pid := int32(cmd.Process.Pid) // #nosec G115 -- PID fits in int32

	synctest.Test(t, func(t *testing.T) {
		require.NoError(t, kill(pid))
	})

	require.EqualError(t, cmd.Wait(), "signal: killed")
}

func Test_sortedKeys(t *testing.T) {
	keys := sortedKeys(map[string]string{
		"charlie": "c",
		"alpha":   "a",
		"bravo":   "b",
	})

	assert.Equal(t, []string{"alpha", "bravo", "charlie"}, keys)
}
