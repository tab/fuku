package readiness

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/process"
	"fuku/internal/contracts"
)

func Test_Checker_checkLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	log := slog.New(slog.DiscardHandler)

	checker := NewChecker(mockPublisher, log)

	tests := []struct {
		name     string
		before   func(t *testing.T) (context.Context, *io.PipeReader, *io.PipeReader, <-chan struct{})
		pattern  string
		timeout  time.Duration
		expected error
	}{
		{
			name: "a matching stdout line is ready",
			before: func(t *testing.T) (context.Context, *io.PipeReader, *io.PipeReader, <-chan struct{}) {
				stdout, stdoutWriter := io.Pipe()
				stderr, stderrWriter := io.Pipe()

				go func() {
					defer stdoutWriter.Close()
					defer stderrWriter.Close()

					stdoutWriter.Write([]byte("Server is starting...\nServer ready on port 8080\n"))
				}()

				return t.Context(), stdout, stderr, make(chan struct{})
			},
			pattern: "ready",
			timeout: 2 * time.Second,
		},
		{
			name: "a matching stderr line is ready",
			before: func(t *testing.T) (context.Context, *io.PipeReader, *io.PipeReader, <-chan struct{}) {
				stdout, stdoutWriter := io.Pipe()
				stderr, stderrWriter := io.Pipe()

				go func() {
					defer stdoutWriter.Close()
					defer stderrWriter.Close()

					stderrWriter.Write([]byte("Server ready on port 8080\n"))
				}()

				return t.Context(), stdout, stderr, make(chan struct{})
			},
			pattern: "ready",
			timeout: 2 * time.Second,
		},
		{
			name: "a line longer than the default scanner buffer is scanned past",
			before: func(t *testing.T) (context.Context, *io.PipeReader, *io.PipeReader, <-chan struct{}) {
				stdout, stdoutWriter := io.Pipe()
				stderr, stderrWriter := io.Pipe()

				go func() {
					defer stdoutWriter.Close()
					defer stderrWriter.Close()

					stdoutWriter.Write([]byte(strings.Repeat("x", 100*1024) + "\nServer ready on port 8080\n"))
				}()

				return t.Context(), stdout, stderr, make(chan struct{})
			},
			pattern: "ready",
			timeout: 2 * time.Second,
		},
		{
			name: "a negative timeout has already elapsed",
			before: func(t *testing.T) (context.Context, *io.PipeReader, *io.PipeReader, <-chan struct{}) {
				stdout, stdoutWriter := io.Pipe()
				stderr, stderrWriter := io.Pipe()

				t.Cleanup(func() { stdoutWriter.Close(); stderrWriter.Close() })

				return t.Context(), stdout, stderr, make(chan struct{})
			},
			pattern:  "ready",
			timeout:  -time.Second,
			expected: contracts.ErrReadinessTimeout,
		},
		{
			name: "a cancelled context stops the scan",
			before: func(t *testing.T) (context.Context, *io.PipeReader, *io.PipeReader, <-chan struct{}) {
				stdout, stdoutWriter := io.Pipe()
				stderr, stderrWriter := io.Pipe()

				t.Cleanup(func() { stdoutWriter.Close(); stderrWriter.Close() })

				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				return ctx, stdout, stderr, make(chan struct{})
			},
			pattern:  "ready",
			timeout:  10 * time.Second,
			expected: context.Canceled,
		},
		{
			name: "an exited process stops the scan",
			before: func(t *testing.T) (context.Context, *io.PipeReader, *io.PipeReader, <-chan struct{}) {
				stdout, stdoutWriter := io.Pipe()
				stderr, stderrWriter := io.Pipe()

				t.Cleanup(func() { stdoutWriter.Close(); stderrWriter.Close() })

				done := make(chan struct{})
				close(done)

				return t.Context(), stdout, stderr, done
			},
			pattern:  "ready",
			timeout:  5 * time.Second,
			expected: contracts.ErrProcessExited,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, stdout, stderr, done := tt.before(t)

			err := checker.checkLog(ctx, tt.pattern, stdout, stderr, tt.timeout, done)

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Checker_checkLog_EndsOnTheTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockPublisher := NewMockPublisher(ctrl)

		log := slog.New(slog.DiscardHandler)

		checker := NewChecker(mockPublisher, log)

		stdout, stdoutWriter := io.Pipe()
		stderr, stderrWriter := io.Pipe()

		go func() {
			defer stdoutWriter.Close()
			defer stderrWriter.Close()

			stdoutWriter.Write([]byte("Server is starting...\n"))
		}()

		err := checker.checkLog(t.Context(), "ready", stdout, stderr, 50*time.Millisecond, make(chan struct{}))

		require.ErrorIs(t, err, contracts.ErrReadinessTimeout)
	})
}

func Test_Checker_checkLog_InvalidPattern(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockPublisher := NewMockPublisher(ctrl)

		log := slog.New(slog.DiscardHandler)

		checker := NewChecker(mockPublisher, log)

		stdout, stdoutWriter := io.Pipe()
		stderr, stderrWriter := io.Pipe()
		line := []byte("Server ready on port 8080\n")

		err := checker.checkLog(t.Context(), "[invalid(", stdout, stderr, time.Second, make(chan struct{}))

		require.ErrorContains(t, err, "invalid regex pattern")

		_, stdoutErr := stdoutWriter.Write(line)
		_, stderrErr := stderrWriter.Write(line)

		require.ErrorIs(t, stdoutErr, io.ErrClosedPipe)
		require.ErrorIs(t, stderrErr, io.ErrClosedPipe)
	})
}

func Test_Checker_checkLog_LineTooLong(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockPublisher := NewMockPublisher(ctrl)
		mockLog := NewMockLogger(ctrl)

		checker := NewChecker(mockPublisher, mockLog)

		stdout, stdoutWriter := io.Pipe()
		stderr, stderrWriter := io.Pipe()
		line := []byte(strings.Repeat("x", process.MaxLineSize+1024*1024) + "\n")
		done := make(chan struct{})
		writeLineAndExit := func() {
			stdoutWriter.Write(line)
			stdoutWriter.Close()
			stderrWriter.Close()
			close(done)
		}

		go writeLineAndExit()

		mockLog.EXPECT().Warn("Log readiness scan ended before a match", "error", bufio.ErrTooLong)

		err := checker.checkLog(t.Context(), "ready", stdout, stderr, time.Second, done)

		require.ErrorIs(t, err, contracts.ErrProcessExited)
	})
}

func Test_Checker_checkLog_NoScannerOutlivesTheCheck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockPublisher := NewMockPublisher(ctrl)

		log := slog.New(slog.DiscardHandler)

		checker := NewChecker(mockPublisher, log)

		stdout, stdoutWriter := io.Pipe()
		stderr, stderrWriter := io.Pipe()
		ready := []byte("Server ready on port 8080\n")
		noise := []byte("GET /health 200\n")
		writeReady := func() {
			stdoutWriter.Write(ready)
		}

		go writeReady()

		err := checker.checkLog(t.Context(), "ready", stdout, stderr, time.Second, make(chan struct{}))

		require.NoError(t, err)

		written, err := stderrWriter.Write(noise)

		require.ErrorIs(t, err, io.ErrClosedPipe)
		assert.Zero(t, written)
	})
}

func Test_Checker_checkLog_DropsALineScannedAfterTheCheck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockPublisher := NewMockPublisher(ctrl)

		log := slog.New(slog.DiscardHandler)

		checker := NewChecker(mockPublisher, log)

		stdout := io.NopCloser(strings.NewReader("Server ready on port 8080\n"))
		stderrReader, stderrWriter := io.Pipe()
		stderr := io.NopCloser(stderrReader)
		late := []byte("GET /health 200\n")
		next := []byte("GET /metrics 200\n")
		unread := make(chan error, 1)
		writeNext := func() {
			_, err := stderrWriter.Write(next)
			unread <- err
		}

		err := checker.checkLog(t.Context(), "ready", stdout, stderr, time.Second, make(chan struct{}))

		require.NoError(t, err)

		_, err = stderrWriter.Write(late)

		require.NoError(t, err)

		go writeNext()

		synctest.Wait()
		stderrReader.Close()

		require.ErrorIs(t, <-unread, io.ErrClosedPipe)
	})
}
