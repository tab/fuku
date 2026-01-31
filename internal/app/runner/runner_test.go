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

	"fuku/internal/app/bus"
	"fuku/internal/app/discovery"
	"fuku/internal/app/errors"
	"fuku/internal/app/logs"
	"fuku/internal/app/process"
	"fuku/internal/app/registry"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewRunner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("RUNNER").Return(componentLog)

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockGuard := NewMockGuard(ctrl)
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockService, mockGuard, mockWorkerPool, mockBus, mockServer, mockLog)

	assert.NotNil(t, r)
	instance, ok := r.(*runner)
	assert.True(t, ok)
	assert.Equal(t, cfg, instance.cfg)
	assert.Equal(t, componentLog, instance.log)
	assert.Equal(t, mockDiscovery, instance.discovery)
	assert.Equal(t, mockGuard, instance.guard)
	assert.Equal(t, mockService, instance.service)
	assert.Equal(t, mockWorkerPool, instance.pool)
	assert.Equal(t, mockRegistry, instance.registry)
	assert.Equal(t, mockBus, instance.bus)
}

func Test_Run_ProfileNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)

	mockLog.EXPECT().WithComponent("RUNNER").Return(componentLog)
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("nonexistent").Return(nil, errors.ErrProfileNotFound)

	mockGuard := NewMockGuard(ctrl)
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockService := NewMockService(ctrl)

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Start(gomock.Any(), "nonexistent").Return(nil)
	mockServer.EXPECT().Stop().Return(nil)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockService, mockGuard, mockWorkerPool, mockBus, mockServer, mockLog)
	ctx := context.Background()

	err := r.Run(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve profile")
}

func Test_Run_ServiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("RUNNER").Return(componentLog)
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()
	componentLog.EXPECT().Debug().Return(nil).AnyTimes()

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("test").Return(nil, errors.ErrServiceNotFound)

	mockGuard := NewMockGuard(ctrl)
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockService := NewMockService(ctrl)

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Start(gomock.Any(), "test").Return(nil)
	mockServer.EXPECT().Stop().Return(nil)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockService, mockGuard, mockWorkerPool, mockBus, mockServer, mockLog)
	ctx := context.Background()

	err := r.Run(ctx, "test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve profile")
}

func Test_Run_SuccessfulStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)

	mockLog.EXPECT().WithComponent("RUNNER").Return(componentLog)
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()
	componentLog.EXPECT().Debug().Return(nil).AnyTimes()

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("test").Return([]discovery.Tier{{Name: "platform", Services: []string{"api"}}}, nil)

	mockGuard := NewMockGuard(ctrl)
	mockGuard.EXPECT().Unlock(gomock.Any()).AnyTimes()

	mockProcess := process.NewMockProcess(ctrl)
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

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockRegistry.EXPECT().Remove(gomock.Any(), gomock.Any()).Return(registry.RemoveResult{Removed: true, Tier: "platform", UnexpectedExit: true}).AnyTimes()
	mockRegistry.EXPECT().Detach(gomock.Any()).AnyTimes()
	mockRegistry.EXPECT().SnapshotReverse().Return([]process.Process{mockProcess}).AnyTimes()
	mockRegistry.EXPECT().Wait().AnyTimes()

	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Start(gomock.Any(), "test").Return(nil)
	mockServer.EXPECT().Stop().Return(nil)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockService, mockGuard, mockWorkerPool, mockBus, mockServer, mockLog)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.Run(ctx, "test")
	assert.NoError(t, err)
}

func Test_Run_NoServices_ExitsGracefully(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)

	mockLog.EXPECT().WithComponent("RUNNER").Return(componentLog)
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Debug().Return(nil).AnyTimes()

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("default").Return([]discovery.Tier{}, nil)

	mockGuard := NewMockGuard(ctrl)
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockService := NewMockService(ctrl)

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Start(gomock.Any(), "default").Return(nil)
	mockServer.EXPECT().Stop().Return(nil)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockService, mockGuard, mockWorkerPool, mockBus, mockServer, mockLog)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.Run(ctx, "default")
	require.NoError(t, err)
}

func Test_StartServiceWithRetry_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := process.NewMockProcess(ctrl)
	readyChan := make(chan error, 1)
	close(readyChan)
	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil)

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}
	ctx := context.Background()

	proc, err := r.startServiceWithRetry(ctx, "api", "default", cfg.Services["api"])

	require.NoError(t, err)
	assert.NotNil(t, proc)
}

func Test_StartServiceWithRetry_ContextCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{
		Dir: "api",
		Readiness: &config.Readiness{
			Type:    config.TypeHTTP,
			URL:     "http://localhost:8080",
			Timeout: 30 * time.Second,
		},
	}
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := process.NewMockProcess(ctrl)

	readyChan := make(chan error)

	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil)
	mockService.EXPECT().Stop(mockProcess).Return(nil)

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
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

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(nil, fmt.Errorf("start failed"))

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
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
	assert.Less(t, elapsed, cfg.Retry.Backoff, "Should cancel immediately without waiting full backoff")
}

func Test_Shutdown_StopsAllProcesses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)

	mockProcess1 := process.NewMockProcess(ctrl)
	mockProcess1.EXPECT().Name().Return("service1").AnyTimes()

	mockProcess2 := process.NewMockProcess(ctrl)
	mockProcess2.EXPECT().Name().Return("service2").AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockProcess1).Return(nil)
	mockService.EXPECT().Stop(mockProcess2).Return(nil)

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]process.Process{mockProcess1, mockProcess2})
	mockRegistry.EXPECT().Detach("service1")
	mockRegistry.EXPECT().Detach("service2")
	mockRegistry.EXPECT().Wait()

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	r.shutdown(mockRegistry)
}

func Test_Shutdown_StopsProcessesOnce(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()

	mockLog := logger.NewMockLogger(ctrl)

	mockProcess1 := process.NewMockProcess(ctrl)
	mockProcess1.EXPECT().Name().Return("service1").AnyTimes()

	mockProcess2 := process.NewMockProcess(ctrl)
	mockProcess2.EXPECT().Name().Return("service2").AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockProcess1).Return(nil).Times(1)
	mockService.EXPECT().Stop(mockProcess2).Return(nil).Times(1)

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]process.Process{mockProcess1, mockProcess2})
	mockRegistry.EXPECT().Detach("service1")
	mockRegistry.EXPECT().Detach("service2")
	mockRegistry.EXPECT().Wait()

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	r.shutdown(mockRegistry)
}

func Test_Shutdown_EmptyRegistry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockService := NewMockService(ctrl)
	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]process.Process{})
	mockRegistry.EXPECT().Wait()

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	r.shutdown(mockRegistry)
}

func Test_HandleCommand_StopService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	doneChan := make(chan struct{})
	close(doneChan)

	mockProcess := process.NewMockProcess(ctrl)
	mockProcess.EXPECT().Done().Return(doneChan)

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockProcess).Return(nil)

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Proc: mockProcess, Tier: "platform", Exists: true, Detached: false})
	mockRegistry.EXPECT().Detach("api")
	mockRegistry.EXPECT().Remove("api", mockProcess).Return(registry.RemoveResult{Removed: true, Tier: "platform", UnexpectedExit: false})

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}
	ctx := context.Background()
	cmd := bus.Message{Type: bus.CommandStopService, Data: bus.Payload{Name: "api"}}

	result := r.handleCommand(ctx, cmd, mockRegistry)

	assert.False(t, result)
}

func Test_HandleCommand_RestartService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api", Tier: "platform"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	oldDoneChan := make(chan struct{})
	close(oldDoneChan)

	mockOldProcess := process.NewMockProcess(ctrl)
	mockOldProcess.EXPECT().Name().Return("api").AnyTimes()
	mockOldProcess.EXPECT().Done().Return(oldDoneChan)

	newDoneChan := make(chan struct{})
	close(newDoneChan)

	mockNewProcess := process.NewMockProcess(ctrl)
	mockNewProcess.EXPECT().Done().Return(newDoneChan).AnyTimes()
	mockNewProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	close(readyChan)
	mockNewProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockNewProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockOldProcess).Return(nil)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockNewProcess, nil)

	done := make(chan struct{})
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Proc: mockOldProcess, Tier: "platform", Exists: true, Detached: false})
	mockRegistry.EXPECT().Detach("api")
	mockRegistry.EXPECT().Remove("api", mockOldProcess).Return(registry.RemoveResult{Removed: true, Tier: "platform", UnexpectedExit: false})
	mockRegistry.EXPECT().Add("api", mockNewProcess, "platform").Do(func(_, _, _ any) { close(done) })
	mockRegistry.EXPECT().Remove("api", mockNewProcess).Return(registry.RemoveResult{Removed: true, Tier: "platform", UnexpectedExit: true}).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	cmd := bus.Message{Type: bus.CommandRestartService, Data: bus.Payload{Name: "api"}}
	result := r.handleCommand(ctx, cmd, mockRegistry)

	assert.False(t, result)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("restart did not complete in time")
	}
}

func Test_HandleCommand_StopAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}
	ctx := context.Background()
	cmd := bus.Message{Type: bus.CommandStopAll}

	result := r.handleCommand(ctx, cmd, registry.NewMockRegistry(ctrl))

	assert.True(t, result)
}

func Test_HandleCommand_InvalidStopServiceData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}
	ctx := context.Background()
	cmd := bus.Message{Type: bus.CommandStopService, Data: "invalid"}

	result := r.handleCommand(ctx, cmd, registry.NewMockRegistry(ctrl))

	assert.False(t, result)
}

func Test_HandleCommand_InvalidRestartServiceData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}
	ctx := context.Background()
	cmd := bus.Message{Type: bus.CommandRestartService, Data: "invalid"}

	result := r.handleCommand(ctx, cmd, registry.NewMockRegistry(ctrl))

	assert.False(t, result)
}

func Test_StopService_ServiceExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	doneChan := make(chan struct{})
	close(doneChan)

	mockProcess := process.NewMockProcess(ctrl)
	mockProcess.EXPECT().Done().Return(doneChan)

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockProcess).Return(nil)

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Proc: mockProcess, Tier: "platform", Exists: true, Detached: false})
	mockRegistry.EXPECT().Detach("api")
	mockRegistry.EXPECT().Remove("api", mockProcess).Return(registry.RemoveResult{Removed: true, Tier: "platform", UnexpectedExit: false})

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	r.stopService("api", mockRegistry)
}

func Test_StopService_ServiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Warn().Return(nil).AnyTimes()

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Proc: nil, Exists: false, Detached: false})

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	r.stopService("api", mockRegistry)
}

func Test_RestartService_ExistingService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api", Tier: "platform"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	oldDoneChan := make(chan struct{})
	close(oldDoneChan)

	mockOldProcess := process.NewMockProcess(ctrl)
	mockOldProcess.EXPECT().Name().Return("api").AnyTimes()
	mockOldProcess.EXPECT().Done().Return(oldDoneChan)

	newDoneChan := make(chan struct{})
	close(newDoneChan)

	mockNewProcess := process.NewMockProcess(ctrl)
	mockNewProcess.EXPECT().Done().Return(newDoneChan).AnyTimes()
	mockNewProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	close(readyChan)
	mockNewProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockNewProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockOldProcess).Return(nil)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockNewProcess, nil)

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Proc: mockOldProcess, Tier: "platform", Exists: true, Detached: false})
	mockRegistry.EXPECT().Detach("api")
	mockRegistry.EXPECT().Remove("api", mockOldProcess).Return(registry.RemoveResult{Removed: true, Tier: "platform", UnexpectedExit: false})
	mockRegistry.EXPECT().Add("api", mockNewProcess, "platform")
	mockRegistry.EXPECT().Remove("api", mockNewProcess).Return(registry.RemoveResult{Removed: true, Tier: "platform", UnexpectedExit: true}).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	r.restartService(ctx, "api", mockRegistry)
}

func Test_RestartService_StoppedService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	doneChan := make(chan struct{})

	mockNewProcess := process.NewMockProcess(ctrl)
	mockNewProcess.EXPECT().Done().Return(doneChan).AnyTimes()
	mockNewProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	close(readyChan)
	mockNewProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockNewProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockNewProcess, nil)

	removeCalled := make(chan struct{})
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Proc: nil, Exists: false, Detached: false})
	mockRegistry.EXPECT().Add("api", mockNewProcess, config.Default)
	mockRegistry.EXPECT().Remove("api", mockNewProcess).DoAndReturn(func(name string, proc process.Process) registry.RemoveResult {
		close(removeCalled)
		return registry.RemoveResult{Removed: true, Tier: config.Default, UnexpectedExit: true}
	})

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	r.restartService(ctx, "api", mockRegistry)

	close(doneChan)

	select {
	case <-removeCalled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("watchProcess goroutine did not call Remove")
	}
}

func Test_RestartService_ConfigNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Error().Return(nil).AnyTimes()
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	mockRegistry := registry.NewMockRegistry(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	r.restartService(ctx, "api", mockRegistry)
}

func Test_RestartService_StartFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Error().Return(nil).AnyTimes()
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(nil, fmt.Errorf("start failed")).Times(cfg.Retry.Attempts)

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Proc: nil, Exists: false, Detached: false})

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	r.restartService(ctx, "api", mockRegistry)
}

func Test_RunServicePhase_CommandStopAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Logs.Buffer = 10

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       b,
		server:    mockServer,
		log:       mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgChan := b.Subscribe(ctx)
	sigChan := make(chan os.Signal, 1)
	mockRegistry := registry.NewMockRegistry(ctrl)

	go func() {
		time.Sleep(5 * time.Millisecond)
		b.Publish(bus.Message{Type: bus.CommandStopAll})
	}()

	r.runServicePhase(ctx, cancel, sigChan, mockRegistry, msgChan)
}

func Test_RunServicePhase_ContextCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	commandChan := make(chan bus.Message)
	sigChan := make(chan os.Signal, 1)
	mockRegistry := registry.NewMockRegistry(ctrl)

	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	r.runServicePhase(ctx, cancel, sigChan, mockRegistry, commandChan)
}

func Test_RunServicePhase_CommandChannelClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	commandChan := make(chan bus.Message)
	sigChan := make(chan os.Signal, 1)
	mockRegistry := registry.NewMockRegistry(ctrl)

	go func() {
		time.Sleep(5 * time.Millisecond)
		close(commandChan)
	}()

	r.runServicePhase(ctx, cancel, sigChan, mockRegistry, commandChan)
}

func Test_StartTier_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := process.NewMockProcess(ctrl)
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

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Add("api", mockProcess, "platform")

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}
	ctx := context.Background()

	failedServices := r.startTier(ctx, "platform", []string{"api"}, mockRegistry)

	assert.Empty(t, failedServices)
}

func Test_StartTier_AcquireError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Error().Return(nil)

	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(context.Canceled)

	mockRegistry := registry.NewMockRegistry(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}
	ctx := context.Background()

	failedServices := r.startTier(ctx, "platform", []string{"api"}, mockRegistry)

	assert.Len(t, failedServices, 1)
	assert.Contains(t, failedServices, "api")
}

func Test_StartTier_ServiceStartupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()
	mockLog.EXPECT().Error().Return(nil)

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(nil, fmt.Errorf("start failed")).Times(cfg.Retry.Attempts)

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorkerPool.EXPECT().Release()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	mockRegistry := registry.NewMockRegistry(ctrl)
	failedServices := r.startTier(ctx, "platform", []string{"api"}, mockRegistry)

	assert.Len(t, failedServices, 1)
	assert.Contains(t, failedServices, "api")
}

func Test_RunStartupPhase_TierWithFailures(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Warn().Return(nil).AnyTimes()
	mockLog.EXPECT().Error().Return(nil)

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(nil, fmt.Errorf("start failed")).Times(cfg.Retry.Attempts)

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorkerPool.EXPECT().Release()

	mockRegistry := registry.NewMockRegistry(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	commandChan := make(chan bus.Message)

	tiers := []discovery.Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, mockRegistry, sigChan, commandChan)

	assert.NoError(t, err)
}

func Test_RunStartupPhase_SignalDuringStartup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := process.NewMockProcess(ctrl)
	doneChan := make(chan struct{})
	mockProcess.EXPECT().Done().Return(doneChan).AnyTimes()
	mockProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).DoAndReturn(
		func(ctx context.Context, name string, srv *config.Service) (process.Process, error) {
			time.Sleep(50 * time.Millisecond)
			return mockProcess, nil
		})
	mockService.EXPECT().Stop(mockProcess).Return(nil).AnyTimes()

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorkerPool.EXPECT().Release()

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockRegistry.EXPECT().Detach(gomock.Any()).AnyTimes()
	mockRegistry.EXPECT().SnapshotReverse().Return([]process.Process{mockProcess}).AnyTimes()
	mockRegistry.EXPECT().Wait().AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	commandChan := make(chan bus.Message)

	go func() {
		time.Sleep(10 * time.Millisecond)

		sigChan <- syscall.SIGTERM
	}()

	tiers := []discovery.Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, mockRegistry, sigChan, commandChan)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "startup interrupted")
}

func Test_RunStartupPhase_ContextCancelledDuringStartup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Warn().Return(nil).AnyTimes()
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).DoAndReturn(
		func(ctx context.Context, name string, srv *config.Service) (process.Process, error) {
			time.Sleep(100 * time.Millisecond)
			return nil, ctx.Err()
		}).AnyTimes()

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil).AnyTimes()
	mockWorkerPool.EXPECT().Release().AnyTimes()

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]process.Process{}).AnyTimes()
	mockRegistry.EXPECT().Wait().AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	commandChan := make(chan bus.Message)

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	tiers := []discovery.Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, mockRegistry, sigChan, commandChan)

	assert.Error(t, err)
}

func Test_RunStartupPhase_CommandChannelClosedDuringStartup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Warn().Return(nil).AnyTimes()
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).DoAndReturn(
		func(ctx context.Context, name string, srv *config.Service) (process.Process, error) {
			time.Sleep(100 * time.Millisecond)
			return nil, ctx.Err()
		}).AnyTimes()

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil).AnyTimes()
	mockWorkerPool.EXPECT().Release().AnyTimes()

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]process.Process{}).AnyTimes()
	mockRegistry.EXPECT().Wait().AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	commandChan := make(chan bus.Message)

	go func() {
		time.Sleep(10 * time.Millisecond)
		close(commandChan)
	}()

	tiers := []discovery.Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, mockRegistry, sigChan, commandChan)

	assert.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrCommandChannelClosed)
}

func Test_RunStartupPhase_StopAllCommandDuringStartup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Warn().Return(nil).AnyTimes()
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).DoAndReturn(
		func(ctx context.Context, name string, srv *config.Service) (process.Process, error) {
			time.Sleep(100 * time.Millisecond)
			return nil, ctx.Err()
		}).AnyTimes()

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil).AnyTimes()
	mockWorkerPool.EXPECT().Release().AnyTimes()

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]process.Process{}).AnyTimes()
	mockRegistry.EXPECT().Wait().AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	commandChan := make(chan bus.Message, 1)

	go func() {
		time.Sleep(10 * time.Millisecond)

		commandChan <- bus.Message{Type: bus.CommandStopAll}
	}()

	tiers := []discovery.Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, mockRegistry, sigChan, commandChan)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "startup interrupted")
}

func Test_RunStartupPhase_OtherCommandDuringStartup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()
	mockLog.EXPECT().Warn().Return(nil).AnyTimes()

	mockProcess := process.NewMockProcess(ctrl)
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

	removeCalled := make(chan struct{})
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Add(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockRegistry.EXPECT().Get("other").Return(registry.Lookup{Exists: false}).AnyTimes()
	mockRegistry.EXPECT().Remove("api", mockProcess).DoAndReturn(func(name string, proc process.Process) registry.RemoveResult {
		close(removeCalled)
		return registry.RemoveResult{Removed: true, Tier: "platform", UnexpectedExit: true}
	})

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	commandChan := make(chan bus.Message, 1)

	commandChan <- bus.Message{Type: bus.CommandStopService, Data: bus.Payload{Name: "other"}}

	tiers := []discovery.Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, mockRegistry, sigChan, commandChan)

	assert.NoError(t, err)

	close(doneChan)

	select {
	case <-removeCalled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("watchProcess goroutine did not call Remove")
	}
}

func Test_RunServicePhase_SignalReceived(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	commandChan := make(chan bus.Message)
	sigChan := make(chan os.Signal, 1)
	mockRegistry := registry.NewMockRegistry(ctrl)

	go func() {
		time.Sleep(5 * time.Millisecond)

		sigChan <- syscall.SIGTERM
	}()

	r.runServicePhase(ctx, cancel, sigChan, mockRegistry, commandChan)
}

func Test_StartServiceWithRetry_ReadinessCheckSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{
		Dir: "api",
		Readiness: &config.Readiness{
			Type:    config.TypeHTTP,
			URL:     "http://localhost:8080",
			Timeout: 100 * time.Millisecond,
		},
	}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := process.NewMockProcess(ctrl)

	readyChan := make(chan error, 1)
	readyChan <- nil

	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil)

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	proc, err := r.startServiceWithRetry(ctx, "api", "default", cfg.Services["api"])

	require.NoError(t, err)
	assert.NotNil(t, proc)
}

func Test_StartServiceWithRetry_ReadinessCheckFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{
		Dir: "api",
		Readiness: &config.Readiness{
			Type:    config.TypeHTTP,
			URL:     "http://localhost:8080",
			Timeout: 100 * time.Millisecond,
		},
	}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := process.NewMockProcess(ctrl)
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
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	proc, err := r.startServiceWithRetry(ctx, "api", "default", cfg.Services["api"])

	assert.Error(t, err)
	assert.Nil(t, proc)
}

func Test_IsWatchedService(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Services["watched"] = &config.Service{
		Dir:   "watched",
		Watch: &config.Watch{Include: []string{"*.go"}},
	}
	cfg.Services["unwatched"] = &config.Service{Dir: "unwatched"}

	r := &runner{cfg: cfg}

	assert.True(t, r.isWatchedService("watched"))
	assert.False(t, r.isWatchedService("unwatched"))
	assert.False(t, r.isWatchedService("nonexistent"))
}

func Test_HandleWatchEvent_RestartInProgress(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{
		Dir:   "api",
		Watch: &config.Watch{Include: []string{"*.go"}},
	}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	mockRegistry := registry.NewMockRegistry(ctrl)

	g := NewGuard()
	g.Lock("api")

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		guard:     g,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	r.handleWatchEvent(ctx, "api", []string{"main.go"}, mockRegistry)
}

func Test_HandleWatchEvent_SuccessfulRestart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{
		Dir:   "api",
		Watch: &config.Watch{Include: []string{"*.go"}},
	}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	doneChan := make(chan struct{})

	mockNewProcess := process.NewMockProcess(ctrl)
	mockNewProcess.EXPECT().Done().Return(doneChan).AnyTimes()
	mockNewProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	close(readyChan)
	mockNewProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockNewProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockNewProcess, nil)

	removeCalled := make(chan struct{})
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Proc: nil, Exists: false, Detached: false})
	mockRegistry.EXPECT().Add("api", mockNewProcess, config.Default)
	mockRegistry.EXPECT().Remove("api", mockNewProcess).DoAndReturn(func(name string, proc process.Process) registry.RemoveResult {
		close(removeCalled)
		return registry.RemoveResult{Removed: true, Tier: config.Default, UnexpectedExit: true}
	})

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		guard:     NewGuard(),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	r.handleWatchEvent(ctx, "api", []string{"main.go"}, mockRegistry)

	close(doneChan)

	select {
	case <-removeCalled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("watchProcess goroutine did not call Remove")
	}
}

func Test_RestartWatchedService_ConfigNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	mockRegistry := registry.NewMockRegistry(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		guard:     NewGuard(),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	r.restartWatchedService(ctx, "api", mockRegistry)
}

func Test_RestartWatchedService_StartFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{
		Dir:   "api",
		Watch: &config.Watch{Include: []string{"*.go"}},
	}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()
	mockLog.EXPECT().Error().Return(nil).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(nil, fmt.Errorf("start failed"))

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Proc: nil, Exists: false, Detached: false})

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		guard:     NewGuard(),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	r.restartWatchedService(ctx, "api", mockRegistry)
}

func Test_RestartWatchedService_ExistingProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{
		Dir:   "api",
		Tier:  "platform",
		Watch: &config.Watch{Include: []string{"*.go"}},
	}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	oldDoneChan := make(chan struct{})
	close(oldDoneChan)

	mockOldProcess := process.NewMockProcess(ctrl)
	mockOldProcess.EXPECT().Done().Return(oldDoneChan)

	newDoneChan := make(chan struct{})

	mockNewProcess := process.NewMockProcess(ctrl)
	mockNewProcess.EXPECT().Done().Return(newDoneChan).AnyTimes()
	mockNewProcess.EXPECT().Name().Return("api").AnyTimes()

	readyChan := make(chan error, 1)
	close(readyChan)
	mockNewProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockCmd := &exec.Cmd{Process: &os.Process{Pid: 12345}}
	mockNewProcess.EXPECT().Cmd().Return(mockCmd).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockOldProcess).Return(nil)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockNewProcess, nil)

	removeCalled := make(chan struct{})
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Get("api").Return(registry.Lookup{Proc: mockOldProcess, Tier: "platform", Exists: true, Detached: false})
	mockRegistry.EXPECT().Detach("api")
	mockRegistry.EXPECT().Remove("api", mockOldProcess).Return(registry.RemoveResult{Removed: true, Tier: "platform", UnexpectedExit: false})
	mockRegistry.EXPECT().Add("api", mockNewProcess, "platform")
	mockRegistry.EXPECT().Remove("api", mockNewProcess).DoAndReturn(func(name string, proc process.Process) registry.RemoveResult {
		close(removeCalled)
		return registry.RemoveResult{Removed: true, Tier: "platform", UnexpectedExit: true}
	})

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  mockRegistry,
		guard:     NewGuard(),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	r.restartWatchedService(ctx, "api", mockRegistry)

	close(newDoneChan)

	select {
	case <-removeCalled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("watchProcess goroutine did not call Remove")
	}
}
