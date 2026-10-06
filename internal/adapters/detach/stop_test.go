package detach

import (
	"bufio"
	"context"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
)

// startProcess runs a script that prints ready once set up, reaped by the test so it never lingers as a zombie
func startProcess(t *testing.T, script string) (int, <-chan struct{}) {
	t.Helper()

	cmd := exec.Command("sh", "-c", script)

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())

	ready, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "ready\n", ready)

	done := make(chan struct{})

	go func() {
		defer close(done)

		cmd.Wait()
	}()

	t.Cleanup(func() {
		cmd.Process.Kill()
		<-done
	})

	return cmd.Process.Pid, done
}

func Test_Stopper_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSocket := NewMockSocket(ctrl)

	reachErr := errors.New("failed to read the status: i/o timeout")
	requestErr := errors.New("failed to read the stop acknowledgement: EOF")

	processes, err := process.Processes()
	require.NoError(t, err)

	unsignalable := slices.IndexFunc(processes, func(p *process.Process) bool {
		return p.Pid > 1 && errors.Is(syscall.Kill(int(p.Pid), 0), syscall.EPERM)
	})

	var exited <-chan struct{}

	tests := []struct {
		name           string
		skip           bool
		before         func()
		ctx            func() context.Context
		timeout        time.Duration
		expectedErr    error
		causes         []error
		expectedOutput string
	}{
		{
			name: "no running instance leaves the stop to the cleanup",
			before: func() {
				mockSocket.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)
			},
			expectedOutput: `^$`,
		},
		{
			name: "an instance that cannot be reached fails the stop",
			before: func() {
				mockSocket.EXPECT().Status().Return(contracts.LogStatus{}, reachErr)
			},
			expectedErr:    reachErr,
			expectedOutput: `^$`,
		},
		{
			name: "an instance without a process ID cannot be signalled",
			before: func() {
				mockSocket.EXPECT().Status().Return(contracts.LogStatus{Profile: "core"}, nil)
			},
			expectedOutput: `^The running fuku reports no process ID; restart it with this version to stop it\n$`,
		},
		{
			name: "an instance that accepts the stop request is waited on without a signal",
			before: func() {
				var pid int

				pid, exited = startProcess(t, `trap "" TERM; trap "exit 0" USR1; echo ready; while :; do sleep 0.1; done`)
				mockSocket.EXPECT().Status().Return(contracts.LogStatus{Profile: "core", PID: pid}, nil)
				mockSocket.EXPECT().RequestStop().DoAndReturn(func() error {
					return syscall.Kill(pid, syscall.SIGUSR1)
				})
			},
			timeout:        5 * time.Second,
			expectedOutput: `^Stopping fuku · pid \d+ · profile core \.\.\. stopped in \d+\.\ds\n$`,
		},
		{
			name: "an older instance that refuses the stop request exits on SIGTERM",
			before: func() {
				var pid int

				pid, exited = startProcess(t, "echo ready; exec sleep 30")
				mockSocket.EXPECT().Status().Return(contracts.LogStatus{Profile: "core", PID: pid}, nil)
				mockSocket.EXPECT().RequestStop().Return(requestErr)
			},
			timeout:        5 * time.Second,
			expectedOutput: `^Stopping fuku · pid \d+ · profile core \.\.\. stopped in \d+\.\ds\n$`,
		},
		{
			name: "an instance that refuses the request and may not be signalled fails the stop",
			skip: unsignalable < 0,
			before: func() {
				mockSocket.EXPECT().Status().Return(contracts.LogStatus{Profile: "core", PID: int(processes[unsignalable].Pid)}, nil)
				mockSocket.EXPECT().RequestStop().Return(requestErr)
			},
			expectedErr:    syscall.EPERM,
			causes:         []error{syscall.EPERM, requestErr},
			expectedOutput: `^Stopping fuku · pid \d+ · profile core \.\.\. failed\n$`,
		},
		{
			name: "init is never signalled",
			before: func() {
				mockSocket.EXPECT().Status().Return(contracts.LogStatus{Profile: "core", PID: 1}, nil)
			},
			expectedOutput: `^The running fuku reports no process ID; restart it with this version to stop it\n$`,
		},
		{
			name: "a negative process ID never signals a process group",
			before: func() {
				mockSocket.EXPECT().Status().Return(contracts.LogStatus{Profile: "core", PID: -99999999}, nil)
			},
			expectedOutput: `^The running fuku reports no process ID; restart it with this version to stop it\n$`,
		},
		{
			name: "an instance that exits before the signal is stopped",
			before: func() {
				mockSocket.EXPECT().Status().Return(contracts.LogStatus{Profile: "core", PID: 99999999}, nil)
				mockSocket.EXPECT().RequestStop().Return(requestErr)
			},
			expectedOutput: `^Stopping fuku · pid \d+ · profile core \.\.\. stopped in \d+\.\ds\n$`,
		},
		{
			name: "an interrupted wait leaves the instance to the caller",
			before: func() {
				var pid int

				pid, exited = startProcess(t, `trap "" TERM; echo ready; while :; do sleep 0.1; done`)
				mockSocket.EXPECT().Status().Return(contracts.LogStatus{Profile: "core", PID: pid}, nil)
				mockSocket.EXPECT().RequestStop().Return(nil)

				exited = nil
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			},
			timeout:        time.Minute,
			expectedErr:    context.Canceled,
			expectedOutput: `^Stopping fuku · pid \d+ · profile core \.\.\. interrupted\n$`,
		},
		{
			name: "an instance that accepts the request but never exits is killed after the timeout",
			before: func() {
				var pid int

				pid, exited = startProcess(t, `trap "" TERM; echo ready; while :; do sleep 0.1; done`)
				mockSocket.EXPECT().Status().Return(contracts.LogStatus{Profile: "core", PID: pid}, nil)
				mockSocket.EXPECT().RequestStop().Return(nil)
			},
			timeout:        300 * time.Millisecond,
			expectedOutput: `^Stopping fuku · pid \d+ · profile core \.\.\. killed after 0\.3s\n$`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("needs a process this user may not signal")
			}

			exited = nil

			tt.before()

			var stdout strings.Builder

			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx()
			}

			err := NewStopper(mockSocket, StopOptions{Timeout: tt.timeout}, &stdout).Stop(ctx)

			require.ErrorIs(t, err, tt.expectedErr)

			for _, cause := range tt.causes {
				require.ErrorIs(t, err, cause)
			}

			assert.Regexp(t, tt.expectedOutput, stdout.String())

			if exited == nil {
				return
			}

			select {
			case <-exited:
			case <-time.After(time.Second):
				t.Fatal("the instance is still running after the stop")
			}
		})
	}
}

func Test_exited(t *testing.T) {
	running := exec.Command("sleep", "30")
	require.NoError(t, running.Start())

	t.Cleanup(func() {
		running.Process.Kill()
		running.Wait()
	})

	zombie := exec.Command("true")
	require.NoError(t, zombie.Start())

	t.Cleanup(func() {
		zombie.Wait()
	})

	tests := []struct {
		name     string
		pid      int
		expected bool
	}{
		{
			name:     "a running process has not exited",
			pid:      running.Process.Pid,
			expected: false,
		},
		{
			name:     "a process the user may not signal has not exited",
			pid:      1,
			expected: false,
		},
		{
			name:     "a zombie its parent has not reaped has exited",
			pid:      zombie.Process.Pid,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Eventually(t, func() bool { return exited(tt.pid) == tt.expected }, 2*time.Second, 10*time.Millisecond)
		})
	}
}
