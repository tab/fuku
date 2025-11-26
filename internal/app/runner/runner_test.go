package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
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

func Test_NewRunner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}

	mockLogger := logger.NewMockLogger(ctrl)
	mockDiscovery := NewMockDiscovery(ctrl)
	mockRegistry := NewMockRegistry(ctrl)
	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)

	mockEvent := runtime.NewNoOpEventBus()
	mockCommand := runtime.NewNoOpCommandBus()

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockService, mockWorkerPool, mockEvent, mockCommand, mockLogger)
	assert.NotNil(t, r)

	instance, ok := r.(*runner)
	assert.True(t, ok)
	assert.Equal(t, cfg, instance.cfg)
	assert.Equal(t, mockLogger, instance.log)
	assert.Equal(t, mockDiscovery, instance.discovery)
	assert.Equal(t, mockService, instance.service)
	assert.Equal(t, mockWorkerPool, instance.pool)
	assert.Equal(t, mockRegistry, instance.registry)
	assert.Equal(t, mockEvent, instance.event)
	assert.Equal(t, mockCommand, instance.command)
}

func Test_Run_ProfileNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	mockDiscovery := NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("nonexistent").Return(nil, errors.ErrProfileNotFound)

	mockRegistry := NewMockRegistry(ctrl)
	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockEvent := runtime.NewNoOpEventBus()
	mockCommand := runtime.NewNoOpCommandBus()

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockService, mockWorkerPool, mockEvent, mockCommand, mockLogger)

	ctx := context.Background()
	err := r.Run(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve profile")
}

func Test_Run_ServiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	mockDiscovery := NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("test").Return(nil, errors.ErrServiceNotFound)

	mockRegistry := NewMockRegistry(ctrl)
	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockEvent := runtime.NewNoOpEventBus()
	mockCommand := runtime.NewNoOpCommandBus()

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockService, mockWorkerPool, mockEvent, mockCommand, mockLogger)

	ctx := context.Background()
	err := r.Run(ctx, "test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve profile")
}

func Test_Run_SuccessfulStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()

	mockDiscovery := NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("test").Return([]Tier{{Name: "platform", Services: []string{"api"}}}, nil)

	mockProcess := NewMockProcess(ctrl)
	doneChan := make(chan struct{})
	close(doneChan)
	mockProcess.EXPECT().Done().Return(doneChan).AnyTimes()
	mockProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	close(readyChan)
	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil)
	mockService.EXPECT().Stop(mockProcess).Return(nil).AnyTimes()

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).AnyTimes()
	mockWorkerPool.EXPECT().Release().AnyTimes()

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockRegistry.EXPECT().SnapshotReverse().Return([]Process{mockProcess}).AnyTimes()
	mockRegistry.EXPECT().Wait().AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.Run(ctx, "test")
	assert.NoError(t, err)
}

func Test_Run_NoServices_ExitsGracefully(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil)
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()

	mockDiscovery := NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("default").Return([]Tier{}, nil)

	mockRegistry := NewMockRegistry(ctrl)
	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		registry:  mockRegistry,
		service:   mockService,
		pool:      mockWorkerPool,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.Run(ctx, "default")
	require.NoError(t, err)
}

func Test_StartServiceWithRetry_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := NewMockProcess(ctrl)
	readyChan := make(chan error, 1)
	close(readyChan)
	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil)

	mockDiscovery := NewMockDiscovery(ctrl)
	mockRegistry := NewMockRegistry(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	proc, err := r.startServiceWithRetry(ctx, "api", "default", cfg.Services["api"])

	require.NoError(t, err)
	assert.NotNil(t, proc)
}

func Test_StartServiceWithRetry_ContextCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {
				Dir: "api",
				Readiness: &config.Readiness{
					Type:    config.TypeHTTP,
					URL:     "http://localhost:8080",
					Timeout: 30 * time.Second,
				},
			},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := NewMockProcess(ctrl)
	readyChan := make(chan error)
	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil)
	mockService.EXPECT().Stop(mockProcess).Return(nil)

	mockDiscovery := NewMockDiscovery(ctrl)
	mockRegistry := NewMockRegistry(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	proc, err := r.startServiceWithRetry(ctx, "api", "default", cfg.Services["api"])

	assert.Error(t, err)
	assert.Nil(t, proc)
	assert.Equal(t, context.Canceled, err)
}

func Test_StartServiceWithRetry_CancellationDuringBackoff(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(nil, fmt.Errorf("start failed"))

	mockDiscovery := NewMockDiscovery(ctrl)
	mockRegistry := NewMockRegistry(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	proc, err := r.startServiceWithRetry(ctx, "api", "default", cfg.Services["api"])
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Nil(t, proc)
	assert.Equal(t, context.Canceled, err)
	assert.Less(t, elapsed, config.RetryBackoff, "Should cancel immediately without waiting full backoff")
}

func Test_Shutdown_StopsAllProcesses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}

	mockLogger := logger.NewMockLogger(ctrl)

	mockProcess1 := NewMockProcess(ctrl)
	mockProcess1.EXPECT().Name().Return("service1").AnyTimes()

	mockProcess2 := NewMockProcess(ctrl)
	mockProcess2.EXPECT().Name().Return("service2").AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockProcess1).Return(nil)
	mockService.EXPECT().Stop(mockProcess2).Return(nil)

	mockDiscovery := NewMockDiscovery(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]Process{mockProcess1, mockProcess2})
	mockRegistry.EXPECT().Wait()

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	r.shutdown(mockRegistry)
}

func Test_Shutdown_StopsProcessesOnce(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}

	mockLogger := logger.NewMockLogger(ctrl)

	mockProcess1 := NewMockProcess(ctrl)
	mockProcess1.EXPECT().Name().Return("service1").AnyTimes()

	mockProcess2 := NewMockProcess(ctrl)
	mockProcess2.EXPECT().Name().Return("service2").AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockProcess1).Return(nil).Times(1)
	mockService.EXPECT().Stop(mockProcess2).Return(nil).Times(1)

	mockDiscovery := NewMockDiscovery(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]Process{mockProcess1, mockProcess2})
	mockRegistry.EXPECT().Wait()

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	r.shutdown(mockRegistry)
}

func Test_Shutdown_EmptyRegistry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}

	mockLogger := logger.NewMockLogger(ctrl)

	mockService := NewMockService(ctrl)
	mockDiscovery := NewMockDiscovery(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]Process{})
	mockRegistry.EXPECT().Wait()

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	r.shutdown(mockRegistry)
}

func Test_HandleCommand_StopService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := NewMockProcess(ctrl)
	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockProcess).Return(nil)

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(Lookup{Proc: mockProcess, Exists: true, Detached: false})
	mockRegistry.EXPECT().Detach("api")

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	cmd := runtime.Command{Type: runtime.CommandStopService, Data: runtime.StopServiceData{Service: "api"}}
	result := r.handleCommand(ctx, cmd, mockRegistry)

	assert.False(t, result)
}

func Test_HandleCommand_RestartService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api", Tier: "platform"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockOldProcess := NewMockProcess(ctrl)
	mockOldProcess.EXPECT().Name().Return("api").AnyTimes()

	doneChan := make(chan struct{})
	close(doneChan)

	mockNewProcess := NewMockProcess(ctrl)
	mockNewProcess.EXPECT().Done().Return(doneChan).AnyTimes()
	mockNewProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	close(readyChan)
	mockNewProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockNewProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockOldProcess).Return(nil)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockNewProcess, nil)

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(Lookup{Proc: mockOldProcess, Exists: true, Detached: false})
	mockRegistry.EXPECT().Detach("api")
	mockRegistry.EXPECT().Add("api", mockNewProcess, "platform")

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	cmd := runtime.Command{Type: runtime.CommandRestartService, Data: runtime.RestartServiceData{Service: "api"}}
	result := r.handleCommand(ctx, cmd, mockRegistry)

	assert.False(t, result)
}

func Test_HandleCommand_StopAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  NewMockRegistry(ctrl),
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	cmd := runtime.Command{Type: runtime.CommandStopAll}
	result := r.handleCommand(ctx, cmd, NewMockRegistry(ctrl))

	assert.True(t, result)
}

func Test_HandleCommand_InvalidStopServiceData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  NewMockRegistry(ctrl),
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	cmd := runtime.Command{Type: runtime.CommandStopService, Data: "invalid"}
	result := r.handleCommand(ctx, cmd, NewMockRegistry(ctrl))

	assert.False(t, result)
}

func Test_HandleCommand_InvalidRestartServiceData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  NewMockRegistry(ctrl),
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	cmd := runtime.Command{Type: runtime.CommandRestartService, Data: "invalid"}
	result := r.handleCommand(ctx, cmd, NewMockRegistry(ctrl))

	assert.False(t, result)
}

func Test_StopService_ServiceExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := NewMockProcess(ctrl)
	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockProcess).Return(nil)

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(Lookup{Proc: mockProcess, Exists: true, Detached: false})
	mockRegistry.EXPECT().Detach("api")

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	r.stopService("api", mockRegistry)
}

func Test_StopService_ServiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(Lookup{Proc: nil, Exists: false, Detached: false})

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	r.stopService("api", mockRegistry)
}

func Test_RestartService_ExistingService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api", Tier: "platform"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockOldProcess := NewMockProcess(ctrl)
	mockOldProcess.EXPECT().Name().Return("api").AnyTimes()

	doneChan := make(chan struct{})
	close(doneChan)

	mockNewProcess := NewMockProcess(ctrl)
	mockNewProcess.EXPECT().Done().Return(doneChan).AnyTimes()
	mockNewProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	close(readyChan)
	mockNewProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockNewProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockOldProcess).Return(nil)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockNewProcess, nil)

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(Lookup{Proc: mockOldProcess, Exists: true, Detached: false})
	mockRegistry.EXPECT().Detach("api")
	mockRegistry.EXPECT().Add("api", mockNewProcess, "platform")

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	r.restartService(ctx, "api", mockRegistry)
}

func Test_RestartService_StoppedService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	doneChan := make(chan struct{})
	close(doneChan)

	mockNewProcess := NewMockProcess(ctrl)
	mockNewProcess.EXPECT().Done().Return(doneChan).AnyTimes()
	mockNewProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	close(readyChan)
	mockNewProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockNewProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockNewProcess, nil)

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(Lookup{Proc: nil, Exists: false, Detached: false})
	mockRegistry.EXPECT().Add("api", mockNewProcess, config.Default)

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	r.restartService(ctx, "api", mockRegistry)
}

func Test_RestartService_ConfigNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(Lookup{Proc: nil, Exists: false, Detached: false})

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	r.restartService(ctx, "api", mockRegistry)
}

func Test_RestartService_StartFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(nil, fmt.Errorf("start failed")).Times(config.RetryAttempt)

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(Lookup{Proc: nil, Exists: false, Detached: false})

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	r.restartService(ctx, "api", mockRegistry)
}

func Test_RunServicePhase_CommandStopAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	commandBus := runtime.NewCommandBus(10)
	defer commandBus.Close()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  NewMockRegistry(ctrl),
		event:     runtime.NewNoOpEventBus(),
		command:   commandBus,
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	commandChan := commandBus.Subscribe(ctx)
	sigChan := make(chan os.Signal, 1)
	mockRegistry := NewMockRegistry(ctrl)

	go func() {
		time.Sleep(5 * time.Millisecond)
		commandBus.Publish(runtime.Command{Type: runtime.CommandStopAll})
	}()

	r.runServicePhase(ctx, cancel, sigChan, mockRegistry, commandChan)
}

func Test_RunServicePhase_ContextCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  NewMockRegistry(ctrl),
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	commandChan := make(chan runtime.Command)
	sigChan := make(chan os.Signal, 1)
	mockRegistry := NewMockRegistry(ctrl)

	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	r.runServicePhase(ctx, cancel, sigChan, mockRegistry, commandChan)
}

func Test_RunServicePhase_CommandChannelClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	mockLogger := logger.NewMockLogger(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  NewMockRegistry(ctrl),
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	commandChan := make(chan runtime.Command)
	sigChan := make(chan os.Signal, 1)
	mockRegistry := NewMockRegistry(ctrl)

	go func() {
		time.Sleep(5 * time.Millisecond)
		close(commandChan)
	}()

	r.runServicePhase(ctx, cancel, sigChan, mockRegistry, commandChan)
}

func Test_StartTier_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := NewMockProcess(ctrl)
	doneChan := make(chan struct{})
	mockProcess.EXPECT().Done().Return(doneChan).AnyTimes()
	mockProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	close(readyChan)
	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil)

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorkerPool.EXPECT().Release()

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Add("api", mockProcess, "platform")

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	err := r.startTier(ctx, "platform", []string{"api"}, mockRegistry)

	assert.NoError(t, err)
}

func Test_StartTier_AcquireError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)

	mockService := NewMockService(ctrl)

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(context.Canceled)

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  NewMockRegistry(ctrl),
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	mockRegistry := NewMockRegistry(ctrl)
	err := r.startTier(ctx, "platform", []string{"api"}, mockRegistry)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service 'api'")
	assert.Contains(t, err.Error(), "failed to acquire worker")
}

func Test_StartTier_ServiceStartupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(nil, fmt.Errorf("start failed")).Times(config.RetryAttempt)

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorkerPool.EXPECT().Release()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  NewMockRegistry(ctrl),
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	mockRegistry := NewMockRegistry(ctrl)
	err := r.startTier(ctx, "platform", []string{"api"}, mockRegistry)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service 'api'")
}

func Test_RunStartupPhase_StartupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(nil, fmt.Errorf("start failed")).Times(config.RetryAttempt)

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorkerPool.EXPECT().Release()

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]Process{})
	mockRegistry.EXPECT().Wait()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	commandChan := make(chan runtime.Command)

	tiers := []Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, mockRegistry, sigChan, commandChan)

	assert.Error(t, err)
}

func Test_RunStartupPhase_SignalDuringStartup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := NewMockProcess(ctrl)
	doneChan := make(chan struct{})
	mockProcess.EXPECT().Done().Return(doneChan).AnyTimes()
	mockProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).DoAndReturn(
		func(ctx context.Context, name string, srv *config.Service) (Process, error) {
			time.Sleep(50 * time.Millisecond)
			return mockProcess, nil
		})
	mockService.EXPECT().Stop(mockProcess).Return(nil).AnyTimes()

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorkerPool.EXPECT().Release()

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockRegistry.EXPECT().SnapshotReverse().Return([]Process{mockProcess}).AnyTimes()
	mockRegistry.EXPECT().Wait().AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	commandChan := make(chan runtime.Command)

	go func() {
		time.Sleep(10 * time.Millisecond)

		sigChan <- syscall.SIGTERM
	}()

	tiers := []Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, mockRegistry, sigChan, commandChan)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "startup interrupted")
}

func Test_RunStartupPhase_ContextCancelledDuringStartup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).DoAndReturn(
		func(ctx context.Context, name string, srv *config.Service) (Process, error) {
			time.Sleep(100 * time.Millisecond)
			return nil, ctx.Err()
		}).AnyTimes()

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil).AnyTimes()
	mockWorkerPool.EXPECT().Release().AnyTimes()

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]Process{}).AnyTimes()
	mockRegistry.EXPECT().Wait().AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	commandChan := make(chan runtime.Command)

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	tiers := []Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, mockRegistry, sigChan, commandChan)

	assert.Error(t, err)
}

func Test_RunStartupPhase_CommandChannelClosedDuringStartup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).DoAndReturn(
		func(ctx context.Context, name string, srv *config.Service) (Process, error) {
			time.Sleep(100 * time.Millisecond)
			return nil, ctx.Err()
		}).AnyTimes()

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil).AnyTimes()
	mockWorkerPool.EXPECT().Release().AnyTimes()

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]Process{}).AnyTimes()
	mockRegistry.EXPECT().Wait().AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	commandChan := make(chan runtime.Command)

	go func() {
		time.Sleep(10 * time.Millisecond)
		close(commandChan)
	}()

	tiers := []Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, mockRegistry, sigChan, commandChan)

	assert.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrCommandChannelClosed)
}

func Test_RunStartupPhase_StopAllCommandDuringStartup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).DoAndReturn(
		func(ctx context.Context, name string, srv *config.Service) (Process, error) {
			time.Sleep(100 * time.Millisecond)
			return nil, ctx.Err()
		}).AnyTimes()

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil).AnyTimes()
	mockWorkerPool.EXPECT().Release().AnyTimes()

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]Process{}).AnyTimes()
	mockRegistry.EXPECT().Wait().AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	commandChan := make(chan runtime.Command, 1)

	go func() {
		time.Sleep(10 * time.Millisecond)

		commandChan <- runtime.Command{Type: runtime.CommandStopAll}
	}()

	tiers := []Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, mockRegistry, sigChan, commandChan)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "startup interrupted")
}

func Test_RunStartupPhase_OtherCommandDuringStartup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()

	mockProcess := NewMockProcess(ctrl)
	doneChan := make(chan struct{})
	close(doneChan)
	mockProcess.EXPECT().Done().Return(doneChan).AnyTimes()
	mockProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	close(readyChan)
	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil)

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorkerPool.EXPECT().Release()

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockRegistry.EXPECT().Get("other").Return(Lookup{Exists: false}).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	commandChan := make(chan runtime.Command, 1)

	commandChan <- runtime.Command{Type: runtime.CommandStopService, Data: runtime.StopServiceData{Service: "other"}}

	tiers := []Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, mockRegistry, sigChan, commandChan)

	assert.NoError(t, err)
}

func Test_RunServicePhase_SignalReceived(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  NewMockRegistry(ctrl),
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	commandChan := make(chan runtime.Command)
	sigChan := make(chan os.Signal, 1)
	mockRegistry := NewMockRegistry(ctrl)

	go func() {
		time.Sleep(5 * time.Millisecond)

		sigChan <- syscall.SIGTERM
	}()

	r.runServicePhase(ctx, cancel, sigChan, mockRegistry, commandChan)
}

func Test_StartServiceWithRetry_ReadinessCheckSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {
				Dir: "api",
				Readiness: &config.Readiness{
					Type:    config.TypeHTTP,
					URL:     "http://localhost:8080",
					Timeout: 100 * time.Millisecond,
				},
			},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := NewMockProcess(ctrl)

	readyChan := make(chan error, 1)
	readyChan <- nil

	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil)

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  NewMockRegistry(ctrl),
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx := context.Background()
	proc, err := r.startServiceWithRetry(ctx, "api", "default", cfg.Services["api"])

	require.NoError(t, err)
	assert.NotNil(t, proc)
}

func Test_StartServiceWithRetry_ReadinessCheckFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {
				Dir: "api",
				Readiness: &config.Readiness{
					Type:    config.TypeHTTP,
					URL:     "http://localhost:8080",
					Timeout: 100 * time.Millisecond,
				},
			},
		},
	}

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := NewMockProcess(ctrl)
	mockProcess.EXPECT().Ready().DoAndReturn(func() <-chan error {
		ch := make(chan error, 1)
		ch <- fmt.Errorf("readiness check failed")

		return ch
	}).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil).AnyTimes()
	mockService.EXPECT().Stop(mockProcess).Return(nil).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  NewMockRegistry(ctrl),
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
		log:       mockLogger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	proc, err := r.startServiceWithRetry(ctx, "api", "default", cfg.Services["api"])

	assert.Error(t, err)
	assert.Nil(t, proc)
}
