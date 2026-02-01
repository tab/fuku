package readiness

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/errors"
	"fuku/internal/app/process"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_CheckHTTP_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	done := make(chan struct{})
	ctx := context.Background()
	err := checker.CheckHTTP(ctx, server.URL, 5*time.Second, 100*time.Millisecond, done)
	assert.NoError(t, err)
}

func Test_CheckHTTP_Timeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	done := make(chan struct{})
	ctx := context.Background()
	err := checker.CheckHTTP(ctx, server.URL, 50*time.Millisecond, 10*time.Millisecond, done)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "readiness check timed out")
}

func Test_CheckHTTP_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := checker.CheckHTTP(ctx, server.URL, 5*time.Second, 100*time.Millisecond, done)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func Test_CheckHTTP_InvalidURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	done := make(chan struct{})
	ctx := context.Background()
	err := checker.CheckHTTP(ctx, "http://invalid\x00url", 5*time.Second, 100*time.Millisecond, done)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create request")
}

func Test_CheckLog_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	defer stdoutReader.Close()
	defer stderrReader.Close()

	go func() {
		defer stdoutWriter.Close()

		fmt.Fprintln(stdoutWriter, "Server is starting...")
		fmt.Fprintln(stdoutWriter, "Server ready on port 8080")
	}()

	go func() {
		defer stderrWriter.Close()
	}()

	done := make(chan struct{})
	ctx := context.Background()
	err := checker.CheckLog(ctx, "ready", stdoutReader, stderrReader, 2*time.Second, done)
	assert.NoError(t, err)
}

func Test_CheckLog_Timeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	defer stdoutReader.Close()
	defer stderrReader.Close()

	go func() {
		defer stdoutWriter.Close()

		fmt.Fprintln(stdoutWriter, "Server is starting...")
	}()

	go func() {
		defer stderrWriter.Close()
	}()

	done := make(chan struct{})
	ctx := context.Background()
	err := checker.CheckLog(ctx, "ready", stdoutReader, stderrReader, 50*time.Millisecond, done)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "readiness check timed out")
}

func Test_CheckLog_InvalidPattern(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	stdoutReader, _ := io.Pipe()
	stderrReader, _ := io.Pipe()

	defer stdoutReader.Close()
	defer stderrReader.Close()

	done := make(chan struct{})
	ctx := context.Background()
	err := checker.CheckLog(ctx, "[invalid(", stdoutReader, stderrReader, 1*time.Second, done)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex pattern")
}

func Test_CheckLog_MatchInStderr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	defer stdoutReader.Close()
	defer stderrReader.Close()

	go func() {
		defer stdoutWriter.Close()
	}()
	go func() {
		defer stderrWriter.Close()

		fmt.Fprintln(stderrWriter, "Server ready on port 8080")
	}()

	done := make(chan struct{})
	ctx := context.Background()
	err := checker.CheckLog(ctx, "ready", stdoutReader, stderrReader, 2*time.Second, done)
	assert.NoError(t, err)
}

func Test_CheckLog_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	defer stdoutReader.Close()
	defer stderrReader.Close()

	writerDone := make(chan struct{})

	go func() {
		defer stdoutWriter.Close()
		defer stderrWriter.Close()

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-writerDone:
				return
			case <-ticker.C:
				fmt.Fprintln(stdoutWriter, "still waiting...")
			}
		}
	}()

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())

	cancel()

	err := checker.CheckLog(ctx, "ready", stdoutReader, stderrReader, 10*time.Second, done)

	close(writerDone)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func Test_CheckLog_NegativeDuration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	defer stdoutReader.Close()
	defer stderrReader.Close()

	go func() {
		defer stdoutWriter.Close()
		defer stderrWriter.Close()
	}()

	done := make(chan struct{})
	ctx := context.Background()
	err := checker.CheckLog(ctx, "ready", stdoutReader, stderrReader, -1*time.Second, done)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "readiness check timed out")
}

func Test_Check_HTTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	componentLogger.EXPECT().Info().Return(nil).AnyTimes()

	checker := NewReadiness(mockLogger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	srv := &config.Service{
		Readiness: &config.Readiness{
			Type:     config.TypeHTTP,
			URL:      server.URL,
			Timeout:  5 * time.Second,
			Interval: 100 * time.Millisecond,
		},
	}

	proc := process.NewProcess(process.Params{Name: "test-service"})

	ctx := context.Background()
	checker.Check(ctx, "test-service", srv, proc)

	select {
	case err := <-proc.Ready():
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("readiness check didn't complete")
	}
}

func Test_Check_Log(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	componentLogger.EXPECT().Info().Return(nil).AnyTimes()

	checker := NewReadiness(mockLogger)

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	srv := &config.Service{
		Readiness: &config.Readiness{
			Type:    config.TypeLog,
			Pattern: "ready",
			Timeout: 2 * time.Second,
		},
	}

	proc := process.NewProcess(process.Params{
		Name:         "test-service",
		StdoutReader: stdoutReader,
		StderrReader: stderrReader,
	})

	go func() {
		defer stdoutWriter.Close()
		defer stderrWriter.Close()

		fmt.Fprintln(stdoutWriter, "Server ready on port 8080")
	}()

	ctx := context.Background()
	checker.Check(ctx, "test-service", srv, proc)

	select {
	case err := <-proc.Ready():
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("readiness check didn't complete")
	}
}

func Test_Check_InvalidType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	componentLogger.EXPECT().Info().Return(nil).AnyTimes()
	componentLogger.EXPECT().Error().Return(nil).AnyTimes()

	checker := NewReadiness(mockLogger)

	srv := &config.Service{
		Readiness: &config.Readiness{
			Type:    "invalid",
			Timeout: 1 * time.Second,
		},
	}

	proc := process.NewProcess(process.Params{Name: "test-service"})

	ctx := context.Background()
	checker.Check(ctx, "test-service", srv, proc)

	select {
	case err := <-proc.Ready():
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid readiness type")
	case <-time.After(2 * time.Second):
		t.Fatal("readiness check didn't complete")
	}
}

func Test_CheckHTTP_ProcessExited(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	done := make(chan struct{})
	close(done)

	ctx := context.Background()
	err := checker.CheckHTTP(ctx, server.URL, 5*time.Second, 100*time.Millisecond, done)
	assert.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrProcessExited)
}

func Test_CheckLog_ProcessExited(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	defer stdoutReader.Close()
	defer stderrReader.Close()

	go func() {
		defer stdoutWriter.Close()
		defer stderrWriter.Close()
	}()

	done := make(chan struct{})
	close(done)

	ctx := context.Background()
	err := checker.CheckLog(ctx, "ready", stdoutReader, stderrReader, 5*time.Second, done)
	assert.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrProcessExited)
}

func Test_CheckTCP_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer listener.Close()

	done := make(chan struct{})
	ctx := context.Background()
	err = checker.CheckTCP(ctx, listener.Addr().String(), 5*time.Second, 100*time.Millisecond, done)
	assert.NoError(t, err)
}

func Test_CheckTCP_Timeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	done := make(chan struct{})
	ctx := context.Background()
	err := checker.CheckTCP(ctx, "127.0.0.1:59999", 50*time.Millisecond, 10*time.Millisecond, done)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "readiness check timed out")
}

func Test_CheckTCP_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := checker.CheckTCP(ctx, "127.0.0.1:59999", 5*time.Second, 100*time.Millisecond, done)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func Test_CheckTCP_ProcessExited(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	checker := NewReadiness(mockLogger)

	done := make(chan struct{})
	close(done)

	ctx := context.Background()
	err := checker.CheckTCP(ctx, "127.0.0.1:59999", 5*time.Second, 100*time.Millisecond, done)
	assert.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrProcessExited)
}

func Test_Check_TCP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("READINESS").Return(componentLogger)
	componentLogger.EXPECT().Info().Return(nil).AnyTimes()

	checker := NewReadiness(mockLogger)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer listener.Close()

	srv := &config.Service{
		Readiness: &config.Readiness{
			Type:     config.TypeTCP,
			Address:  listener.Addr().String(),
			Timeout:  5 * time.Second,
			Interval: 100 * time.Millisecond,
		},
	}

	proc := process.NewProcess(process.Params{Name: "test-service"})

	ctx := context.Background()
	checker.Check(ctx, "test-service", srv, proc)

	select {
	case err := <-proc.Ready():
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("readiness check didn't complete")
	}
}
