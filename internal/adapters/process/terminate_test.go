package process

import (
	"io"
	"log/slog"
	"os/exec"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/model"
)

func Test_Handle_Terminate(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api"}

	tests := []struct {
		name   string
		before func(t *testing.T) *Handle
	}{
		{
			name: "a child in its own group exits on SIGTERM",
			before: func(t *testing.T) *Handle {
				cmd := exec.Command("sleep", "10")
				cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
				require.NoError(t, cmd.Start())

				handle := newHandle(svc, cmd, nil, nil, log)

				go func() {
					defer close(handle.done)

					cmd.Wait()
				}()

				return handle
			},
		},
		{
			name: "a child outside its own group is signalled directly",
			before: func(t *testing.T) *Handle {
				cmd := exec.Command("sleep", "10")
				require.NoError(t, cmd.Start())

				handle := newHandle(svc, cmd, nil, nil, log)

				go func() {
					defer close(handle.done)

					cmd.Wait()
				}()

				return handle
			},
		},
		{
			name: "a child that already exited needs nothing",
			before: func(t *testing.T) *Handle {
				cmd := exec.Command("true")
				require.NoError(t, cmd.Start())
				require.NoError(t, cmd.Wait())

				handle := newHandle(svc, cmd, nil, nil, log)
				close(handle.done)

				return handle
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle := tt.before(t)

			err := handle.Terminate()

			require.NoError(t, err)

			select {
			case <-handle.done:
			case <-time.After(2 * time.Second):
				t.Fatal("the child did not exit")
			}
		})
	}
}

func Test_Handle_Terminate_KilledAfterTimeout(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api"}

	cmd := exec.Command("sh", "-c", "trap '' TERM INT; echo ready; sleep 60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())

	ready := make([]byte, 6)
	_, err = io.ReadFull(stdout, ready)
	require.NoError(t, err)

	defer stdout.Close()

	pid := cmd.Process.Pid

	synctest.Test(t, func(t *testing.T) {
		handle := newHandle(svc, cmd, nil, nil, log)

		// sync: polls with WNOHANG because a blocking Wait4 would freeze virtual time until the child exits on its own
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			var status syscall.WaitStatus

			for range ticker.C {
				if wpid, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil); err == nil && wpid == pid {
					close(handle.done)

					return
				}
			}
		}()

		start := time.Now()

		err := handle.Terminate()

		require.NoError(t, err)
		assert.GreaterOrEqual(t, time.Since(start), ShutdownTimeout)
	})
}

func Test_Handle_Terminate_ReapedBeforeSignal(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api"}

	cmd := exec.Command("true")
	require.NoError(t, cmd.Start())
	require.NoError(t, cmd.Wait())

	handle := newHandle(svc, cmd, nil, nil, log)

	err := handle.Terminate()

	require.NoError(t, err)
}

func Test_Handle_Terminate_ReleasedProcess(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api"}

	cmd := exec.Command("sleep", "10")
	require.NoError(t, cmd.Start())

	pid := cmd.Process.Pid
	handle := newHandle(svc, cmd, nil, nil, log)

	require.NoError(t, cmd.Process.Release())

	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })

	err := handle.Terminate()

	require.ErrorContains(t, err, "failed to terminate process")
}

func Test_Handle_Terminate_SharedByCallers(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api"}

	cmd := exec.Command("sleep", "10")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, cmd.Start())

	handle := newHandle(svc, cmd, nil, nil, log)

	go func() {
		defer close(handle.done)

		cmd.Wait()
	}()

	results := make(chan error, 2)

	go func() { results <- handle.Terminate() }()
	go func() { results <- handle.Terminate() }()

	first := <-results
	second := <-results
	later := handle.Terminate()

	require.NoError(t, first)
	require.NoError(t, second)
	require.NoError(t, later)

	select {
	case <-handle.done:
	case <-time.After(2 * time.Second):
		t.Fatal("the child did not exit")
	}
}

func Test_Handle_forceKill_Reaped(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api"}

	cmd := exec.Command("true")
	require.NoError(t, cmd.Start())
	require.NoError(t, cmd.Wait())

	handle := newHandle(svc, cmd, nil, nil, log)

	err := handle.forceKill()

	require.NoError(t, err)
}
