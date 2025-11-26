package runner

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/errors"
	"fuku/internal/app/runtime"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLifecycle := NewMockLifecycle(ctrl)
	mockReadiness := NewMockReadiness(ctrl)
	mockEventBus := runtime.NewNoOpEventBus()
	mockLogger := logger.NewMockLogger(ctrl)

	s := NewService(mockLifecycle, mockReadiness, mockEventBus, mockLogger)

	assert.NotNil(t, s)
	instance, ok := s.(*service)
	assert.True(t, ok)
	assert.Equal(t, mockReadiness, instance.readiness)
	assert.Equal(t, mockLifecycle, instance.lifecycle)
	assert.Equal(t, mockLogger, instance.log)
}

func Test_Start_DirectoryNotExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLifecycle := NewMockLifecycle(ctrl)
	mockReadiness := NewMockReadiness(ctrl)
	mockEventBus := runtime.NewNoOpEventBus()
	mockLogger := logger.NewMockLogger(ctrl)

	s := NewService(mockLifecycle, mockReadiness, mockEventBus, mockLogger)

	ctx := context.Background()
	svc := &config.Service{
		Dir: "/nonexistent/directory/path",
	}

	proc, err := s.Start(ctx, "test-service", svc)

	assert.Error(t, err)
	assert.Nil(t, proc)
	assert.ErrorIs(t, err, errors.ErrServiceDirectoryNotExist)
}

func Test_Start_MissingEnvFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLifecycle := NewMockLifecycle(ctrl)
	mockLifecycle.EXPECT().Configure(gomock.Any()).AnyTimes()

	mockReadiness := NewMockReadiness(ctrl)
	mockEventBus := runtime.NewNoOpEventBus()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()

	s := NewService(mockLifecycle, mockReadiness, mockEventBus, mockLogger)

	tmpDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	svc := &config.Service{
		Dir: tmpDir,
	}

	_, err := s.Start(ctx, "test-service", svc)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start command")
}

func Test_Start_RelativePathConversion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLifecycle := NewMockLifecycle(ctrl)
	mockReadiness := NewMockReadiness(ctrl)
	mockEventBus := runtime.NewNoOpEventBus()
	mockLogger := logger.NewMockLogger(ctrl)

	s := NewService(mockLifecycle, mockReadiness, mockEventBus, mockLogger)

	ctx := context.Background()
	svc := &config.Service{
		Dir: "nonexistent",
	}

	proc, err := s.Start(ctx, "test-service", svc)

	assert.Error(t, err)
	assert.Nil(t, proc)
	assert.ErrorIs(t, err, errors.ErrServiceDirectoryNotExist)
}

func Test_Stop_NilProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProcess := NewMockProcess(ctrl)

	mockLifecycle := NewMockLifecycle(ctrl)
	mockLifecycle.EXPECT().Terminate(mockProcess, config.ShutdownTimeout).Return(nil)

	mockReadiness := NewMockReadiness(ctrl)
	mockEventBus := runtime.NewNoOpEventBus()
	mockLogger := logger.NewMockLogger(ctrl)

	s := NewService(mockLifecycle, mockReadiness, mockEventBus, mockLogger)

	err := s.Stop(mockProcess)
	assert.NoError(t, err)
}

func Test_Start_WithValidDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")
	err := os.WriteFile(makefilePath, []byte("run:\n\techo 'test'\n"), 0644)
	require.NoError(t, err)

	mockLifecycle := NewMockLifecycle(ctrl)
	mockLifecycle.EXPECT().Configure(gomock.Any()).AnyTimes()
	mockLifecycle.EXPECT().Terminate(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	mockReadiness := NewMockReadiness(ctrl)
	mockEventBus := runtime.NewNoOpEventBus()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	s := NewService(mockLifecycle, mockReadiness, mockEventBus, mockLogger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := &config.Service{
		Dir: tmpDir,
	}

	proc, err := s.Start(ctx, "test-service", svc)
	if err != nil {
		assert.Contains(t, err.Error(), "failed to start command")
	} else {
		assert.NotNil(t, proc)
		cancel()

		if proc != nil {
			s.Stop(proc)
		}
	}
}

func Test_HandleReadinessCheck_NoReadiness(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	mockReadiness := NewMockReadiness(ctrl)
	mockLifecycle := NewMockLifecycle(ctrl)

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	svc := &service{
		lifecycle: mockLifecycle,
		readiness: mockReadiness,
		log:       mockLogger,
	}

	proc := &process{
		ready: make(chan error, 1),
	}

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stdoutWriter.Close()
	defer stderr.Close()
	defer stderrWriter.Close()

	serviceCfg := &config.Service{
		Dir:       "/tmp/test",
		Readiness: nil,
	}

	svc.handleReadinessCheck(ctx, "test-service", serviceCfg, proc, stdout, stderr)

	select {
	case err := <-proc.ready:
		assert.NoError(t, err, "Process should be signaled as ready immediately when no readiness check is configured")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected ready signal to be sent immediately")
	}
}

func Test_HandleReadinessCheck_HTTPReadiness(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	mockReadiness := NewMockReadiness(ctrl)
	mockLifecycle := NewMockLifecycle(ctrl)

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	checkCalled := make(chan struct{})

	mockReadiness.EXPECT().Check(gomock.Any(), "test-service", gomock.Any(), gomock.Any()).
		Times(1).
		Do(func(_, _, _, _ interface{}) {
			close(checkCalled)
		})

	svc := &service{
		lifecycle: mockLifecycle,
		readiness: mockReadiness,
		log:       mockLogger,
	}

	proc := &process{
		ready: make(chan error, 1),
	}

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stdoutWriter.Close()
	defer stderr.Close()
	defer stderrWriter.Close()

	serviceCfg := &config.Service{
		Dir: "/tmp/test",
		Readiness: &config.Readiness{
			Type: config.TypeHTTP,
			URL:  "http://localhost:8080",
		},
	}

	svc.handleReadinessCheck(ctx, "test-service", serviceCfg, proc, stdout, stderr)

	select {
	case <-checkCalled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected readiness check to be called asynchronously for HTTP type")
	}
}

func Test_HandleReadinessCheck_LogReadiness(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	mockReadiness := NewMockReadiness(ctrl)
	mockLifecycle := NewMockLifecycle(ctrl)

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	checkCalled := make(chan struct{})

	mockReadiness.EXPECT().Check(gomock.Any(), "test-service", gomock.Any(), gomock.Any()).
		Times(1).
		Do(func(_, _, _, _ interface{}) {
			close(checkCalled)
		})

	svc := &service{
		lifecycle: mockLifecycle,
		readiness: mockReadiness,
		log:       mockLogger,
	}

	proc := &process{
		ready: make(chan error, 1),
	}

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stdoutWriter.Close()
	defer stderr.Close()
	defer stderrWriter.Close()

	serviceCfg := &config.Service{
		Dir: "/tmp/test",
		Readiness: &config.Readiness{
			Type:    config.TypeLog,
			Pattern: "Ready",
		},
	}

	svc.handleReadinessCheck(ctx, "test-service", serviceCfg, proc, stdout, stderr)

	select {
	case <-checkCalled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected readiness check to be called asynchronously for Log type")
	}
}

func Test_HandleReadinessCheck_UnknownType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	mockReadiness := NewMockReadiness(ctrl)
	mockLifecycle := NewMockLifecycle(ctrl)

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	svc := &service{
		lifecycle: mockLifecycle,
		readiness: mockReadiness,
		log:       mockLogger,
	}

	proc := &process{
		ready: make(chan error, 1),
	}

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stdoutWriter.Close()
	defer stderr.Close()
	defer stderrWriter.Close()

	serviceCfg := &config.Service{
		Dir: "/tmp/test",
		Readiness: &config.Readiness{
			Type: "unknown",
		},
	}

	svc.handleReadinessCheck(ctx, "test-service", serviceCfg, proc, stdout, stderr)

	select {
	case err := <-proc.ready:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown readiness type")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected ready signal with error for unknown type")
	}
}

func Test_TeeStream_WithOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	eventBus := runtime.NewNoOpEventBus()

	svc := &service{
		event: eventBus,
		log:   mockLogger,
	}

	reader, writer := io.Pipe()
	dstReader, dstWriter := io.Pipe()

	done := make(chan struct{})

	go func() {
		svc.teeStream(reader, dstWriter, "test-service", "platform", "STDOUT")
		close(done)
	}()

	go func() {
		writer.Write([]byte("test line 1\n"))
		writer.Write([]byte("test line 2\n"))
		writer.Close()
	}()

	go func() {
		io.Copy(io.Discard, dstReader)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("teeStream did not complete")
	}
}

func Test_TeeStream_EmptyTier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	eventBus := runtime.NewNoOpEventBus()

	svc := &service{
		event: eventBus,
		log:   mockLogger,
	}

	reader, writer := io.Pipe()
	dstReader, dstWriter := io.Pipe()

	done := make(chan struct{})

	go func() {
		svc.teeStream(reader, dstWriter, "test-service", "", "STDOUT")
		close(done)
	}()

	go func() {
		writer.Write([]byte("test output\n"))
		writer.Close()
	}()

	go func() {
		io.Copy(io.Discard, dstReader)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("teeStream did not complete")
	}
}
