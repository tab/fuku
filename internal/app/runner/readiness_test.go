package runner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_CheckHTTP_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	checker := NewReadiness(mockLogger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	err := checker.CheckHTTP(ctx, server.URL, 5*time.Second, 100*time.Millisecond)
	assert.NoError(t, err)
}

func Test_CheckHTTP_Timeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	checker := NewReadiness(mockLogger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx := context.Background()
	err := checker.CheckHTTP(ctx, server.URL, 200*time.Millisecond, 50*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "readiness check timed out")
}

func Test_CheckHTTP_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	checker := NewReadiness(mockLogger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := checker.CheckHTTP(ctx, server.URL, 5*time.Second, 100*time.Millisecond)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func Test_CheckLog_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
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

	ctx := context.Background()
	err := checker.CheckLog(ctx, "ready", stdoutReader, stderrReader, 2*time.Second)
	assert.NoError(t, err)
}

func Test_CheckLog_Timeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
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

	ctx := context.Background()
	err := checker.CheckLog(ctx, "ready", stdoutReader, stderrReader, 200*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "readiness check timed out")
}

func Test_CheckLog_InvalidPattern(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	checker := NewReadiness(mockLogger)

	stdoutReader, _ := io.Pipe()
	stderrReader, _ := io.Pipe()

	defer stdoutReader.Close()
	defer stderrReader.Close()

	ctx := context.Background()
	err := checker.CheckLog(ctx, "[invalid(", stdoutReader, stderrReader, 1*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex pattern")
}

func Test_CheckLog_MatchInStderr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
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

	ctx := context.Background()
	err := checker.CheckLog(ctx, "ready", stdoutReader, stderrReader, 2*time.Second)
	assert.NoError(t, err)
}

func Test_Check_HTTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

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

	proc := &process{
		name:  "test-service",
		ready: make(chan error, 1),
	}

	ctx := context.Background()
	checker.Check(ctx, "test-service", srv, proc)

	select {
	case err := <-proc.ready:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("readiness check didn't complete")
	}
}

func Test_Check_Log(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

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

	proc := &process{
		name:         "test-service",
		ready:        make(chan error, 1),
		stdoutReader: stdoutReader,
		stderrReader: stderrReader,
	}

	go func() {
		defer stdoutWriter.Close()
		defer stderrWriter.Close()

		time.Sleep(100 * time.Millisecond)
		fmt.Fprintln(stdoutWriter, "Server ready on port 8080")
	}()

	ctx := context.Background()
	checker.Check(ctx, "test-service", srv, proc)

	select {
	case err := <-proc.ready:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("readiness check didn't complete")
	}
}

func Test_Check_InvalidType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	checker := NewReadiness(mockLogger)

	srv := &config.Service{
		Readiness: &config.Readiness{
			Type:    "invalid",
			Timeout: 1 * time.Second,
		},
	}

	proc := &process{
		name:  "test-service",
		ready: make(chan error, 1),
	}

	ctx := context.Background()
	checker.Check(ctx, "test-service", srv, proc)

	select {
	case err := <-proc.ready:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid readiness type")
	case <-time.After(2 * time.Second):
		t.Fatal("readiness check didn't complete")
	}
}
