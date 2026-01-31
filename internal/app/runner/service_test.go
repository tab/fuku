package runner

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/errors"
	"fuku/internal/app/lifecycle"
	"fuku/internal/app/logs"
	"fuku/internal/app/process"
	"fuku/internal/app/readiness"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)
	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockServer := logs.NewMockServer(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("SERVICE").Return(componentLog)

	s := NewService(mockLifecycle, mockReadiness, mockServer, mockLog)

	assert.NotNil(t, s)
	instance, ok := s.(*service)
	assert.True(t, ok)
	assert.Equal(t, mockReadiness, instance.readiness)
	assert.Equal(t, mockLifecycle, instance.lifecycle)
	assert.Equal(t, componentLog, instance.log)
}

func Test_Start_DirectoryNotExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)
	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockServer := logs.NewMockServer(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("SERVICE").Return(componentLog)

	s := NewService(mockLifecycle, mockReadiness, mockServer, mockLog)

	ctx := context.Background()
	svc := &config.Service{
		Dir: "/nonexistent/directory/path",
	}

	proc, err := s.Start(ctx, "test-service", svc)

	assert.Error(t, err)
	assert.Nil(t, proc)
	assert.ErrorIs(t, err, errors.ErrServiceDirectoryNotExist)
}

func Test_Start_EmptyDirectory_MakefileNotFound(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("Skipping: make is not available on this system")
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)
	mockLifecycle.EXPECT().Configure(gomock.Any())
	mockLifecycle.EXPECT().Terminate(gomock.Any(), gomock.Any()).Return(nil)

	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("SERVICE").Return(componentLog)
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	s := NewService(mockLifecycle, mockReadiness, mockServer, mockLog)

	ctx := context.Background()
	svc := &config.Service{Dir: t.TempDir()}

	proc, err := s.Start(ctx, "test-service", svc)

	require.NoError(t, err)
	require.NotNil(t, proc)

	<-proc.Done()
	s.Stop(proc)
}

func Test_Start_EmptyDirectory_MakeNotFound(t *testing.T) {
	if _, err := exec.LookPath("make"); err == nil {
		t.Skip("Skipping: make is available on this system")
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)
	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockServer := logs.NewMockServer(ctrl)

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("SERVICE").Return(componentLog)
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()

	s := NewService(mockLifecycle, mockReadiness, mockServer, mockLog)

	ctx := context.Background()
	svc := &config.Service{Dir: t.TempDir()}

	proc, err := s.Start(ctx, "test-service", svc)

	assert.Error(t, err)
	assert.Nil(t, proc)
	assert.Contains(t, err.Error(), "failed to start command")
}

func Test_Start_RelativePathConversion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)
	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockServer := logs.NewMockServer(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("SERVICE").Return(componentLog)

	s := NewService(mockLifecycle, mockReadiness, mockServer, mockLog)

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

	mockProcess := process.NewMockProcess(ctrl)

	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)
	mockLifecycle.EXPECT().Terminate(mockProcess, config.ShutdownTimeout).Return(nil)

	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockServer := logs.NewMockServer(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("SERVICE").Return(componentLog)

	s := NewService(mockLifecycle, mockReadiness, mockServer, mockLog)

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

	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)
	mockLifecycle.EXPECT().Configure(gomock.Any()).AnyTimes()
	mockLifecycle.EXPECT().Terminate(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("SERVICE").Return(componentLog)
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	s := NewService(mockLifecycle, mockReadiness, mockServer, mockLog)

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

	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	svc := &service{
		lifecycle: mockLifecycle,
		readiness: mockReadiness,
		log:       mockLog,
	}

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stdoutWriter.Close()
	defer stderr.Close()
	defer stderrWriter.Close()

	proc := process.New(process.Params{
		Name:         "test-service",
		StdoutReader: stdout,
		StderrReader: stderr,
	})

	serviceCfg := &config.Service{
		Dir:       "/tmp/test",
		Readiness: nil,
	}

	svc.handleReadinessCheck(ctx, "test-service", serviceCfg, proc, stdout, stderr)

	select {
	case err := <-proc.Ready():
		assert.NoError(t, err, "Process should be signaled as ready immediately when no readiness check is configured")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected ready signal to be sent immediately")
	}
}

func Test_HandleReadinessCheck_HTTPReadiness(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	checkCalled := make(chan struct{})

	mockReadiness.EXPECT().Check(gomock.Any(), "test-service", gomock.Any(), gomock.Any()).
		Times(1).
		Do(func(_, _, _, _ interface{}) {
			close(checkCalled)
		})

	svc := &service{
		lifecycle: mockLifecycle,
		readiness: mockReadiness,
		log:       mockLog,
	}

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stdoutWriter.Close()
	defer stderr.Close()
	defer stderrWriter.Close()

	proc := process.New(process.Params{
		Name:         "test-service",
		StdoutReader: stdout,
		StderrReader: stderr,
	})

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

	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	checkCalled := make(chan struct{})

	mockReadiness.EXPECT().Check(gomock.Any(), "test-service", gomock.Any(), gomock.Any()).
		Times(1).
		Do(func(_, _, _, _ interface{}) {
			close(checkCalled)
		})

	svc := &service{
		lifecycle: mockLifecycle,
		readiness: mockReadiness,
		log:       mockLog,
	}

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stdoutWriter.Close()
	defer stderr.Close()
	defer stderrWriter.Close()

	proc := process.New(process.Params{
		Name:         "test-service",
		StdoutReader: stdout,
		StderrReader: stderr,
	})

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

	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	svc := &service{
		lifecycle: mockLifecycle,
		readiness: mockReadiness,
		log:       mockLog,
	}

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stdoutWriter.Close()
	defer stderr.Close()
	defer stderrWriter.Close()

	proc := process.New(process.Params{
		Name:         "test-service",
		StdoutReader: stdout,
		StderrReader: stderr,
	})

	serviceCfg := &config.Service{
		Dir: "/tmp/test",
		Readiness: &config.Readiness{
			Type: "unknown",
		},
	}

	svc.handleReadinessCheck(ctx, "test-service", serviceCfg, proc, stdout, stderr)

	select {
	case err := <-proc.Ready():
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown readiness type")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected ready signal with error for unknown type")
	}
}

func Test_TeeStream_WithOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	svc := &service{
		log: mockLog,
	}

	reader, writer := io.Pipe()
	dstReader, dstWriter := io.Pipe()

	done := make(chan struct{})

	go func() {
		svc.teeStream(reader, dstWriter, "test-service", "STDOUT")
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

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	svc := &service{
		log: mockLog,
	}

	reader, writer := io.Pipe()
	dstReader, dstWriter := io.Pipe()

	done := make(chan struct{})

	go func() {
		svc.teeStream(reader, dstWriter, "test-service", "STDOUT")
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
