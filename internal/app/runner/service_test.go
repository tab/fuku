package runner

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/app/lifecycle"
	"fuku/internal/app/logs"
	"fuku/internal/app/process"
	"fuku/internal/app/readiness"
	"fuku/internal/app/registry"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)
	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockGuard := NewMockGuard(ctrl)
	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)
	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("SERVICE").Return(componentLog)

	s := NewService(cfg, mockLifecycle, mockReadiness, mockRegistry, mockGuard, mockBus, mockServer, mockLog)

	assert.NotNil(t, s)
	instance, ok := s.(*service)
	assert.True(t, ok)
	assert.Equal(t, cfg, instance.cfg)
	assert.Equal(t, mockLifecycle, instance.lifecycle)
	assert.Equal(t, mockReadiness, instance.readiness)
	assert.Equal(t, mockRegistry, instance.registry)
	assert.Equal(t, mockGuard, instance.guard)
	assert.Equal(t, componentLog, instance.log)
}

func Test_Stop_ServiceNotRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Exists: false})

	s := &service{
		cfg:      cfg,
		registry: mockRegistry,
		bus:      bus.NoOp(),
		log:      mockLog,
	}

	s.Stop("api")
}

func Test_Stop_ServiceRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	doneChan := make(chan struct{})
	close(doneChan)

	mockProcess := process.NewMockProcess(ctrl)
	mockProcess.EXPECT().Done().Return(doneChan)

	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)
	mockLifecycle.EXPECT().Terminate(mockProcess, config.ShutdownTimeout).Return(nil)

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Proc: mockProcess, Tier: "platform", Exists: true})
	mockRegistry.EXPECT().Detach("api")
	mockRegistry.EXPECT().Remove("api", mockProcess).Return(registry.RemoveResult{Removed: true})

	s := &service{
		cfg:       cfg,
		lifecycle: mockLifecycle,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	s.Stop("api")
}

func Test_Restart_GuardLocked(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	mockGuard := NewMockGuard(ctrl)
	mockGuard.EXPECT().Lock("api").Return(false)

	s := &service{
		cfg:   cfg,
		guard: mockGuard,
		bus:   bus.NoOp(),
		log:   mockLog,
	}

	ctx := context.Background()
	s.Restart(ctx, "api")
}

func Test_Restart_ConfigNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	mockGuard := NewMockGuard(ctrl)
	mockGuard.EXPECT().Lock("api").Return(true)
	mockGuard.EXPECT().Unlock("api")

	s := &service{
		cfg:   cfg,
		guard: mockGuard,
		bus:   bus.NoOp(),
		log:   mockLog,
	}

	ctx := context.Background()
	s.Restart(ctx, "api")
}

func Test_Restart_StoppedService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")
	err := os.WriteFile(makefilePath, []byte("run:\n\techo 'test'\n"), 0644)
	require.NoError(t, err)

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: tmpDir, Tier: "platform"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Warn().Return(nil).AnyTimes()
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	mockGuard := NewMockGuard(ctrl)
	mockGuard.EXPECT().Lock("api").Return(true)
	mockGuard.EXPECT().Unlock("api")

	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)
	mockLifecycle.EXPECT().Configure(gomock.Any()).AnyTimes()
	mockLifecycle.EXPECT().Terminate(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Exists: false})
	mockRegistry.EXPECT().Add("api", gomock.Any(), "platform")
	mockRegistry.EXPECT().Remove("api", gomock.Any()).Return(registry.RemoveResult{Removed: true, UnexpectedExit: true}).AnyTimes()

	s := &service{
		cfg:       cfg,
		lifecycle: mockLifecycle,
		readiness: mockReadiness,
		registry:  mockRegistry,
		guard:     mockGuard,
		bus:       bus.NoOp(),
		server:    mockServer,
		log:       mockLog,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s.Restart(ctx, "api")
}

func Test_GetConfig_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	s := &service{cfg: cfg}

	serviceCfg, tier := s.getConfig("nonexistent")

	assert.Nil(t, serviceCfg)
	assert.Empty(t, tier)
}

func Test_GetConfig_Found_WithTier(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api", Tier: "platform"}
	s := &service{cfg: cfg}

	serviceCfg, tier := s.getConfig("api")

	assert.NotNil(t, serviceCfg)
	assert.Equal(t, "platform", tier)
}

func Test_GetConfig_Found_DefaultTier(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}
	s := &service{cfg: cfg}

	serviceCfg, tier := s.getConfig("api")

	assert.NotNil(t, serviceCfg)
	assert.Equal(t, config.Default, tier)
}

func Test_IsWatched(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Services["watched"] = &config.Service{
		Dir:   "watched",
		Watch: &config.Watch{Include: []string{"*.go"}},
	}
	cfg.Services["unwatched"] = &config.Service{Dir: "unwatched"}

	s := &service{cfg: cfg}

	assert.True(t, s.isWatched("watched"))
	assert.False(t, s.isWatched("unwatched"))
	assert.False(t, s.isWatched("nonexistent"))
}

func Test_DoStart_DirectoryNotExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)

	s := &service{
		cfg: cfg,
		log: mockLog,
	}

	ctx := context.Background()
	svc := &config.Service{Dir: "/nonexistent/directory/path"}

	proc, err := s.doStart(ctx, "test-service", "platform", svc)

	assert.Error(t, err)
	assert.Nil(t, proc)
	assert.ErrorIs(t, err, errors.ErrServiceDirectoryNotExist)
}

func Test_DoStart_RelativePathConversion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)

	s := &service{
		cfg: cfg,
		log: mockLog,
	}

	ctx := context.Background()
	svc := &config.Service{Dir: "nonexistent"}

	proc, err := s.doStart(ctx, "test-service", "platform", svc)

	assert.Error(t, err)
	assert.Nil(t, proc)
	assert.ErrorIs(t, err, errors.ErrServiceDirectoryNotExist)
}

func Test_DoStart_ValidDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")
	err := os.WriteFile(makefilePath, []byte("run:\n\techo 'test'\n"), 0644)
	require.NoError(t, err)

	cfg := config.DefaultConfig()

	mockLifecycle := lifecycle.NewMockLifecycle(ctrl)
	mockLifecycle.EXPECT().Configure(gomock.Any()).AnyTimes()
	mockLifecycle.EXPECT().Terminate(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	mockReadiness := readiness.NewMockReadiness(ctrl)
	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Warn().Return(nil).AnyTimes()
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	s := &service{
		cfg:       cfg,
		lifecycle: mockLifecycle,
		readiness: mockReadiness,
		bus:       bus.NoOp(),
		server:    mockServer,
		log:       mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := &config.Service{Dir: tmpDir}

	proc, err := s.doStart(ctx, "test-service", "platform", svc)
	if err != nil {
		assert.Contains(t, err.Error(), "failed to start command")
	} else {
		assert.NotNil(t, proc)
		cancel()

		if proc != nil {
			_ = s.lifecycle.Terminate(proc, config.ShutdownTimeout)
		}
	}
}

func Test_SetupReadinessCheck_NoReadiness(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	s := &service{}

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stdoutWriter.Close()
	defer stderr.Close()
	defer stderrWriter.Close()

	proc := process.NewProcess(process.Params{
		Name:         "test-service",
		StdoutReader: stdout,
		StderrReader: stderr,
	})

	serviceCfg := &config.Service{
		Dir:       "/tmp/test",
		Readiness: nil,
	}

	s.setupReadinessCheck(ctx, "test-service", serviceCfg, proc)

	select {
	case err := <-proc.Ready():
		assert.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected ready signal")
	}
}

func Test_SetupReadinessCheck_HTTPReadiness(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	mockReadiness := readiness.NewMockReadiness(ctrl)

	checkCalled := make(chan struct{})

	mockReadiness.EXPECT().Check(gomock.Any(), "test-service", gomock.Any(), gomock.Any()).
		Times(1).
		Do(func(_, _, _, _ interface{}) {
			close(checkCalled)
		})

	s := &service{
		readiness: mockReadiness,
	}

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stdoutWriter.Close()
	defer stderr.Close()
	defer stderrWriter.Close()

	proc := process.NewProcess(process.Params{
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

	s.setupReadinessCheck(ctx, "test-service", serviceCfg, proc)

	select {
	case <-checkCalled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected readiness check to be called")
	}
}

func Test_SetupReadinessCheck_LogReadiness(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	mockReadiness := readiness.NewMockReadiness(ctrl)

	checkCalled := make(chan struct{})

	mockReadiness.EXPECT().Check(gomock.Any(), "test-service", gomock.Any(), gomock.Any()).
		Times(1).
		Do(func(_, _, _, _ interface{}) {
			close(checkCalled)
		})

	s := &service{
		readiness: mockReadiness,
	}

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stdoutWriter.Close()
	defer stderr.Close()
	defer stderrWriter.Close()

	proc := process.NewProcess(process.Params{
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

	s.setupReadinessCheck(ctx, "test-service", serviceCfg, proc)

	select {
	case <-checkCalled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected readiness check to be called")
	}
}

func Test_SetupReadinessCheck_UnknownType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	s := &service{}

	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	defer stdout.Close()
	defer stdoutWriter.Close()
	defer stderr.Close()
	defer stderrWriter.Close()

	proc := process.NewProcess(process.Params{
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

	s.setupReadinessCheck(ctx, "test-service", serviceCfg, proc)

	select {
	case err := <-proc.Ready():
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown readiness type")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected ready signal with error")
	}
}

func Test_TeeStream_WithOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	s := &service{log: mockLog}

	reader, writer := io.Pipe()
	dstReader, dstWriter := io.Pipe()

	done := make(chan struct{})

	go func() {
		s.teeStream(reader, dstWriter, "test-service", "STDOUT")
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

func Test_WaitForReady_NoReadiness(t *testing.T) {
	s := &service{}
	ctx := context.Background()
	cfg := &config.Service{Readiness: nil}

	err := s.waitForReady(ctx, nil, cfg)

	assert.NoError(t, err)
}

func Test_WaitForReady_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := &service{}
	ctx := context.Background()

	readyChan := make(chan error, 1)
	close(readyChan)

	mockProcess := process.NewMockProcess(ctrl)
	mockProcess.EXPECT().Ready().Return(readyChan)

	cfg := &config.Service{
		Readiness: &config.Readiness{Type: config.TypeHTTP},
	}

	err := s.waitForReady(ctx, mockProcess, cfg)

	assert.NoError(t, err)
}

func Test_WaitForReady_Failed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := &service{}
	ctx := context.Background()

	readyChan := make(chan error, 1)
	readyChan <- errors.ErrReadinessTimeout

	mockProcess := process.NewMockProcess(ctrl)
	mockProcess.EXPECT().Ready().Return(readyChan)

	cfg := &config.Service{
		Readiness: &config.Readiness{Type: config.TypeHTTP},
	}

	err := s.waitForReady(ctx, mockProcess, cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "readiness check failed")
}

func Test_WaitForReady_ContextCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := &service{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	readyChan := make(chan error)

	mockProcess := process.NewMockProcess(ctrl)
	mockProcess.EXPECT().Ready().Return(readyChan)

	cfg := &config.Service{
		Readiness: &config.Readiness{Type: config.TypeHTTP},
	}

	err := s.waitForReady(ctx, mockProcess, cfg)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func Test_ExtractAddress(t *testing.T) {
	s := &service{}

	tests := []struct {
		name      string
		readiness *config.Readiness
		expected  string
	}{
		{
			name:      "nil readiness",
			readiness: nil,
			expected:  "",
		},
		{
			name: "HTTP type with port",
			readiness: &config.Readiness{
				Type: config.TypeHTTP,
				URL:  "http://localhost:8080/health",
			},
			expected: "localhost:8080",
		},
		{
			name: "HTTP type without port",
			readiness: &config.Readiness{
				Type: config.TypeHTTP,
				URL:  "http://localhost/health",
			},
			expected: "localhost:80",
		},
		{
			name: "HTTP type with IP address",
			readiness: &config.Readiness{
				Type: config.TypeHTTP,
				URL:  "http://127.0.0.1:3000/api",
			},
			expected: "127.0.0.1:3000",
		},
		{
			name: "TCP type with address",
			readiness: &config.Readiness{
				Type:    config.TypeTCP,
				Address: "localhost:9090",
			},
			expected: "localhost:9090",
		},
		{
			name: "TCP type with IP address",
			readiness: &config.Readiness{
				Type:    config.TypeTCP,
				Address: "0.0.0.0:8080",
			},
			expected: "0.0.0.0:8080",
		},
		{
			name: "Log type returns empty",
			readiness: &config.Readiness{
				Type:    config.TypeLog,
				Pattern: "ready",
			},
			expected: "",
		},
		{
			name: "unknown type returns empty",
			readiness: &config.Readiness{
				Type: "unknown",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.extractAddress(tt.readiness)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_PreFlightCheck_PortFree(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)

	s := &service{log: mockLog}

	readiness := &config.Readiness{
		Type:    config.TypeTCP,
		Address: "localhost:59999",
	}

	err := s.preFlightCheck("test-service", readiness)

	assert.NoError(t, err)
}

func Test_PreFlightCheck_PortInUse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	defer listener.Close()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Warn().Return(nil).AnyTimes()

	s := &service{log: mockLog}

	err = s.preFlightCheck("test-service", &config.Readiness{
		Type:    config.TypeTCP,
		Address: listener.Addr().String(),
	})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrPortAlreadyInUse))
}

func Test_PreFlightCheck_NilReadiness(t *testing.T) {
	s := &service{}

	err := s.preFlightCheck("test-service", nil)

	assert.NoError(t, err)
}

func Test_PreFlightCheck_NoPort(t *testing.T) {
	s := &service{}

	readiness := &config.Readiness{
		Type:    config.TypeLog,
		Pattern: "ready",
	}

	err := s.preFlightCheck("test-service", readiness)

	assert.NoError(t, err)
}

func Test_ExtractFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "URL with explicit port",
			url:      "http://localhost:8080/health",
			expected: "localhost:8080",
		},
		{
			name:     "URL with IP address and port",
			url:      "http://127.0.0.1:3000/api",
			expected: "127.0.0.1:3000",
		},
		{
			name:     "HTTP URL without port defaults to 80",
			url:      "http://localhost/health",
			expected: "localhost:80",
		},
		{
			name:     "HTTPS URL without port defaults to 443",
			url:      "https://localhost/health",
			expected: "localhost:443",
		},
		{
			name:     "URL with 0.0.0.0",
			url:      "http://0.0.0.0:8080/health",
			expected: "0.0.0.0:8080",
		},
		{
			name:     "with username and password",
			url:      "postgresql://user:password@localhost:5432/database",
			expected: "localhost:5432",
		},
		{
			name:     "invalid URL returns empty",
			url:      "://invalid",
			expected: "",
		},
		{
			name:     "empty URL returns empty",
			url:      "",
			expected: "",
		},
		{
			name:     "unknown scheme without port returns empty",
			url:      "ftp://localhost/file",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFromURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}
