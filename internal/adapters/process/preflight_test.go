package process

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
)

func Test_NewPreflight(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockReporter := NewMockReporter(ctrl)
	mockWorker := NewMockPool(ctrl)

	log := slog.New(slog.DiscardHandler)

	preflight := NewPreflight(mockPublisher, mockReporter, mockWorker, log)

	assert.NotNil(t, preflight)
	assert.Equal(t, mockPublisher, preflight.publisher)
	assert.Equal(t, mockReporter, preflight.reporter)
	assert.Equal(t, mockWorker, preflight.worker)
	assert.NotNil(t, preflight.scan)
	assert.NotNil(t, preflight.kill)
	assert.Equal(t, log, preflight.log)
}

func Test_Preflight_Cleanup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockWorker := NewMockPool(ctrl)
	mockWorker.EXPECT().Acquire(gomock.Any()).Return(nil).AnyTimes()
	mockWorker.EXPECT().Release().AnyTimes()

	log := slog.New(slog.DiscardHandler)

	scanErr := errors.New("permission denied")
	killErr := errors.New("operation not permitted")

	wd, err := os.Getwd()
	require.NoError(t, err)

	started := func(services ...string) contracts.Message {
		return contracts.Message{Type: contracts.EventPreflightStarted, Data: contracts.PreflightStarted{Services: services}}
	}
	killed := func(service, name string, pid int) contracts.Message {
		return contracts.Message{Type: contracts.EventPreflightKilled, Data: contracts.PreflightKilled{Service: service, Name: name, PID: pid}}
	}
	completed := func(killed int) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.PreflightComplete)

			return msg.Type == contracts.EventPreflightComplete && ok && data.Killed == killed
		})
	}

	tests := []struct {
		name          string
		before        func()
		dirs          map[string]string
		processes     []running
		scanErr       error
		killErr       error
		expectedKills int32
		expected      error
	}{
		{
			name:   "no directories publishes nothing",
			before: func() {},
			dirs:   map[string]string{},
		},
		{
			name: "no matching process kills nothing",
			before: func() {
				mockPublisher.EXPECT().Publish(started("api")).Return(nil)
				mockPublisher.EXPECT().Publish(completed(0)).Return(nil)
			},
			dirs: map[string]string{"api": "/project/api"},
			processes: []running{
				{pid: 100, dir: "/other/dir", name: "node"},
			},
		},
		{
			name: "a relative directory is resolved against the working directory",
			before: func() {
				mockPublisher.EXPECT().Publish(started("api")).Return(nil)
				mockPublisher.EXPECT().Publish(killed("api", "node", 100)).Return(nil)
				mockPublisher.EXPECT().Publish(completed(1)).Return(nil)
			},
			dirs: map[string]string{"api": "services/api"},
			processes: []running{
				{pid: 100, dir: filepath.Join(wd, "services/api"), name: "node"},
			},
			expectedKills: 1,
		},
		{
			name: "a matching process is killed and announced",
			before: func() {
				mockPublisher.EXPECT().Publish(started("api")).Return(nil)
				mockPublisher.EXPECT().Publish(killed("api", "node", 100)).Return(nil)
				mockPublisher.EXPECT().Publish(completed(1)).Return(nil)
			},
			dirs: map[string]string{"api": "/project/api"},
			processes: []running{
				{pid: 100, dir: "/project/api", name: "node"},
			},
			expectedKills: 1,
		},
		{
			name: "every service directory is matched exactly",
			before: func() {
				mockPublisher.EXPECT().Publish(started("api", "web")).Return(nil)
				mockPublisher.EXPECT().Publish(killed("api", "node", 100)).Return(nil)
				mockPublisher.EXPECT().Publish(killed("web", "go", 200)).Return(nil)
				mockPublisher.EXPECT().Publish(completed(2)).Return(nil)
			},
			dirs: map[string]string{"web": "/project/web", "api": "/project/api"},
			processes: []running{
				{pid: 100, dir: "/project/api", name: "node"},
				{pid: 150, dir: "/project/api-v2", name: "node"},
				{pid: 200, dir: "/project/web", name: "go"},
				{pid: 300, dir: "/other/dir", name: "vim"},
			},
			expectedKills: 2,
		},
		{
			name: "the own process is never killed",
			before: func() {
				mockPublisher.EXPECT().Publish(started("api")).Return(nil)
				mockPublisher.EXPECT().Publish(killed("api", "node", 200)).Return(nil)
				mockPublisher.EXPECT().Publish(completed(1)).Return(nil)
			},
			dirs: map[string]string{"api": "/project/api"},
			processes: []running{
				{pid: int32(os.Getpid()), dir: "/project/api", name: "fuku"},
				{pid: 200, dir: "/project/api", name: "node"},
			},
			expectedKills: 1,
		},
		{
			name: "a failed scan completes with nothing killed and reports the error",
			before: func() {
				mockPublisher.EXPECT().Publish(started("api")).Return(nil)
				mockPublisher.EXPECT().Publish(completed(0)).Return(nil)
			},
			dirs:     map[string]string{"api": "/project/api"},
			scanErr:  scanErr,
			expected: scanErr,
		},
		{
			name: "a failed kill still counts and moves on",
			before: func() {
				mockPublisher.EXPECT().Publish(started("api", "web")).Return(nil)
				mockPublisher.EXPECT().Publish(killed("api", "node", 100)).Return(nil)
				mockPublisher.EXPECT().Publish(killed("web", "go", 200)).Return(nil)
				mockPublisher.EXPECT().Publish(completed(2)).Return(nil)
			},
			dirs: map[string]string{"api": "/project/api", "web": "/project/web"},
			processes: []running{
				{pid: 100, dir: "/project/api", name: "node"},
				{pid: 200, dir: "/project/web", name: "go"},
			},
			killErr:       killErr,
			expectedKills: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			var kills atomic.Int32

			scan := func() ([]running, error) {
				return tt.processes, tt.scanErr
			}
			kill := func(_ int32) error {
				kills.Add(1)

				return tt.killErr
			}

			preflight := &Preflight{publisher: mockPublisher, worker: mockWorker, scan: scan, kill: kill, log: log}

			err := preflight.Cleanup(t.Context(), tt.dirs)

			require.ErrorIs(t, err, tt.expected)
			assert.Equal(t, tt.expectedKills, kills.Load())
		})
	}
}

func Test_Preflight_Cleanup_CancelledContextStopsKills(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	completed := func(killed int) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			data, ok := msg.Data.(contracts.PreflightComplete)

			return msg.Type == contracts.EventPreflightComplete && ok && data.Killed == killed
		})
	}

	mockPublisher := NewMockPublisher(ctrl)
	mockPublisher.EXPECT().Publish(contracts.Message{
		Type: contracts.EventPreflightStarted,
		Data: contracts.PreflightStarted{Services: []string{"api", "web"}},
	}).Return(nil)
	mockPublisher.EXPECT().Publish(completed(0)).Return(nil)

	mockWorker := NewMockPool(ctrl)
	mockWorker.EXPECT().Acquire(gomock.Any()).Return(context.Canceled)

	log := slog.New(slog.DiscardHandler)

	var kills atomic.Int32

	scan := func() ([]running, error) {
		return []running{
			{pid: 100, dir: "/project/api", name: "node"},
			{pid: 200, dir: "/project/web", name: "go"},
		}, nil
	}
	kill := func(_ int32) error {
		kills.Add(1)

		return nil
	}

	preflight := &Preflight{publisher: mockPublisher, worker: mockWorker, scan: scan, kill: kill, log: log}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := preflight.Cleanup(ctx, map[string]string{"api": "/project/api", "web": "/project/web"})

	require.NoError(t, err)
	assert.Equal(t, int32(0), kills.Load())
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
