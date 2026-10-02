package detach

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_Command_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStarter := NewMockStarter(ctrl)
	mockProcess := NewMockProcess(ctrl)
	mockView := NewMockView(ctrl)

	starting := Record{Kind: KindStarting, Service: "api"}
	ready := Record{Kind: KindReady, Service: "api", Duration: 1200 * time.Millisecond}
	running := Record{Kind: KindRunning, PID: 4242, Count: 1, Duration: 4100 * time.Millisecond}
	served := Record{Kind: KindRunning, PID: 4242, Count: 1, Duration: 4100 * time.Millisecond, Address: "127.0.0.1:9090"}
	failed := Record{Kind: KindFailed, Service: "api", Error: "max retries exceeded"}
	pipe := func(lines ...[]byte) io.Reader {
		return bytes.NewReader(bytes.Join(lines, nil))
	}
	launched := func(output io.Reader) {
		mockStarter.EXPECT().Launch().Return(mockProcess, nil)
		mockProcess.EXPECT().Output().Return(output)
		mockView.EXPECT().Open()
	}

	var cancelRun context.CancelFunc

	launchErr := errors.New("failed to locate the fuku binary")
	signalErr := errors.New("process already finished")

	tests := []struct {
		name           string
		before         func()
		ctx            func() context.Context
		expectedCode   int
		expectedErr    error
		expectedStdout string
		expectedStderr string
	}{
		{
			name: "a failed launch fails the run",
			before: func() {
				mockStarter.EXPECT().Launch().Return(nil, launchErr)
			},
			expectedCode: 1,
			expectedErr:  launchErr,
		},
		{
			name: "a running child is released and summarised",
			before: func() {
				launched(pipe(encode(starting), encode(ready), encode(served)))
				gomock.InOrder(
					mockView.EXPECT().Show(starting),
					mockView.EXPECT().Show(ready),
					mockView.EXPECT().Show(served),
					mockView.EXPECT().Close(),
					mockProcess.EXPECT().Release().Return(nil),
				)
			},
			expectedStdout: "Running detached · pid 4242 · 1 services · 4.1s\nAPI 127.0.0.1:9090\n",
		},
		{
			name: "a running child without an API prints no API line",
			before: func() {
				launched(pipe(encode(running)))
				mockView.EXPECT().Show(running)
				mockView.EXPECT().Close()
				mockProcess.EXPECT().Release().Return(nil)
			},
			expectedStdout: "Running detached · pid 4242 · 1 services · 4.1s\n",
		},
		{
			name: "a failed release fails the run",
			before: func() {
				launched(pipe(encode(running)))
				mockView.EXPECT().Show(running)
				mockView.EXPECT().Close()
				mockProcess.EXPECT().Release().Return(os.ErrProcessDone)
			},
			expectedCode: 1,
			expectedErr:  os.ErrProcessDone,
		},
		{
			name: "a failed start prints what the child printed",
			before: func() {
				launched(pipe(encode(failed), []byte("Error: service 'api' failed to start\n")))
				gomock.InOrder(
					mockView.EXPECT().Show(failed),
					mockProcess.EXPECT().Terminate().Return(os.ErrProcessDone),
					mockProcess.EXPECT().Wait().Return(errors.New("exit status 1")),
					mockView.EXPECT().Close(),
				)
			},
			expectedCode:   1,
			expectedStderr: "Error: service 'api' failed to start\n",
		},
		{
			name: "a child that exits without a word fails the run",
			before: func() {
				launched(pipe())
				mockProcess.EXPECT().Terminate().Return(os.ErrProcessDone)
				mockProcess.EXPECT().Wait().Return(errors.New("signal: killed"))
				mockView.EXPECT().Close()
			},
			expectedCode: 1,
			expectedErr:  errExitedEarly,
		},
		{
			name: "an error the child printed after its running record fails the run",
			before: func() {
				launched(pipe(encode(running), []byte("Error: failed to release the detached start output\n")))
				gomock.InOrder(
					mockView.EXPECT().Show(running),
					mockProcess.EXPECT().Terminate().Return(nil),
					mockProcess.EXPECT().Wait().Return(errors.New("exit status 1")),
					mockView.EXPECT().Close(),
				)
			},
			expectedCode:   1,
			expectedStderr: "Error: failed to release the detached start output\n",
		},
		{
			name: "a line longer than a scanner token still arrives whole",
			before: func() {
				long := Record{Kind: KindProfile, Services: []string{strings.Repeat("s", 100_000)}}

				launched(pipe(encode(long), encode(running)))
				mockView.EXPECT().Show(long)
				mockView.EXPECT().Show(running)
				mockView.EXPECT().Close()
				mockProcess.EXPECT().Release().Return(nil)
			},
			expectedStdout: "Running detached · pid 4242 · 1 services · 4.1s\n",
		},
		{
			name: "a start that completes after the wait was aborted stops the child instead of releasing it",
			before: func() {
				launched(pipe(encode(running)))
				mockView.EXPECT().Show(running).Do(func(Record) { cancelRun() })
				gomock.InOrder(
					mockProcess.EXPECT().Terminate().Return(nil),
					mockProcess.EXPECT().Wait().Return(nil),
					mockView.EXPECT().Close(),
				)
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancelRun = cancel

				return ctx
			},
			expectedCode: exitInterrupted,
		},
		{
			name: "a signal aborts the wait before the coordinator cancels",
			before: func() {
				reader, writer, err := os.Pipe()
				require.NoError(t, err)

				mockStarter.EXPECT().Launch().DoAndReturn(func() (Process, error) {
					require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))

					return mockProcess, nil
				})
				mockProcess.EXPECT().Output().Return(reader)
				mockView.EXPECT().Open()
				gomock.InOrder(
					mockProcess.EXPECT().Terminate().DoAndReturn(func() error {
						return writer.Close()
					}),
					mockProcess.EXPECT().Wait().Return(nil),
					mockView.EXPECT().Close(),
				)
			},
			expectedCode: exitInterrupted,
		},
		{
			name: "an aborted wait stops the child and exits like an interrupt",
			before: func() {
				reader, writer, err := os.Pipe()
				require.NoError(t, err)

				launched(reader)
				gomock.InOrder(
					mockProcess.EXPECT().Terminate().DoAndReturn(func() error {
						return writer.Close()
					}),
					mockProcess.EXPECT().Wait().Return(nil),
					mockView.EXPECT().Close(),
				)
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			},
			expectedCode: exitInterrupted,
		},
		{
			name: "a child that cannot be stopped fails the run",
			before: func() {
				reader, writer, err := os.Pipe()
				require.NoError(t, err)

				t.Cleanup(func() {
					writer.Close()
					reader.Close()
				})

				launched(reader)
				mockProcess.EXPECT().Terminate().Return(signalErr)
				mockView.EXPECT().Close()
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			},
			expectedCode: 1,
			expectedErr:  signalErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			ctx := t.Context()
			if tt.ctx != nil {
				ctx = tt.ctx()
			}

			var stdout, stderr strings.Builder

			code, err := NewCommand(mockStarter, mockView, &stdout, &stderr).Run(ctx)

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedCode, code)

			if tt.expectedStdout != "" {
				assert.True(t, signal.Ignored(os.Interrupt), "a released child is never reported as interrupted")
			}

			signal.Reset(os.Interrupt, syscall.SIGTERM)
			assert.Equal(t, tt.expectedStdout, stdout.String())
			assert.Equal(t, tt.expectedStderr, stderr.String())
		})
	}
}

func Test_aborted(t *testing.T) {
	tests := []struct {
		name      string
		cancelled bool
		signalled bool
		expected  bool
	}{
		{
			name:      "a cancelled context aborts",
			cancelled: true,
			expected:  true,
		},
		{
			name:      "a received signal aborts",
			signalled: true,
			expected:  true,
		},
		{
			name:     "neither lets the wait settle",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			if tt.cancelled {
				cancel()
			}

			interrupt := make(chan os.Signal, 1)
			if tt.signalled {
				interrupt <- os.Interrupt
			}

			assert.Equal(t, tt.expected, aborted(ctx, interrupt))
		})
	}
}
