package runner

import (
	"context"
	"os"
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
	"fuku/internal/app/preflight"
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
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockPreflight := preflight.NewMockPreflight(ctrl)
	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockPreflight, mockService, mockWorkerPool, mockBus, mockServer, mockLog)

	assert.NotNil(t, r)
	instance, ok := r.(*runner)
	assert.True(t, ok)
	assert.Equal(t, cfg, instance.cfg)
	assert.Equal(t, componentLog, instance.log)
	assert.Equal(t, mockDiscovery, instance.discovery)
	assert.Equal(t, mockPreflight, instance.preflight)
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

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockPreflight := preflight.NewMockPreflight(ctrl)
	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockPreflight, mockService, mockWorkerPool, mockBus, mockServer, mockLog)
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

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockPreflight := preflight.NewMockPreflight(ctrl)
	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockPreflight, mockService, mockWorkerPool, mockBus, mockServer, mockLog)
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

	mockPreflight := preflight.NewMockPreflight(ctrl)
	mockPreflight.EXPECT().Cleanup(gomock.Any()).Return(nil, nil)

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", "platform").Return(nil)
	mockService.EXPECT().Stop("api").AnyTimes()

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).AnyTimes()
	mockWorkerPool.EXPECT().Release().AnyTimes()

	mockProcess := process.NewMockProcess(ctrl)
	mockProcess.EXPECT().Name().Return("api").AnyTimes()

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]process.Process{mockProcess}).AnyTimes()
	mockRegistry.EXPECT().Detach("api").AnyTimes()
	mockRegistry.EXPECT().Wait().AnyTimes()

	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Start(gomock.Any(), "test", []string{"api"}).Return(nil)
	mockServer.EXPECT().Stop().Return(nil)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockPreflight, mockService, mockWorkerPool, mockBus, mockServer, mockLog)

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

	mockPreflight := preflight.NewMockPreflight(ctrl)
	mockRegistry := registry.NewMockRegistry(ctrl)
	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockPreflight, mockService, mockWorkerPool, mockBus, mockServer, mockLog)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.Run(ctx, "default")
	require.NoError(t, err)
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
	mockService.EXPECT().Stop("service1")
	mockService.EXPECT().Stop("service2")

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
		preflight: preflight.NewMockPreflight(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	r.shutdown()
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
		preflight: preflight.NewMockPreflight(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  mockRegistry,
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	r.shutdown()
}

func Test_HandleCommand_StopService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop("api")

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		preflight: preflight.NewMockPreflight(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	cmd := bus.Message{Type: bus.CommandStopService, Data: bus.Payload{Name: "api"}}

	result := r.handleCommand(ctx, cmd)

	assert.False(t, result)
}

func Test_HandleCommand_RestartService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api", Tier: "platform"}

	mockLog := logger.NewMockLogger(ctrl)

	done := make(chan struct{})
	mockService := NewMockService(ctrl)
	mockService.EXPECT().Restart(gomock.Any(), "api").Do(func(_ context.Context, _ string) {
		close(done)
	})

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		preflight: preflight.NewMockPreflight(ctrl),
		service:   mockService,
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	cmd := bus.Message{Type: bus.CommandRestartService, Data: bus.Payload{Name: "api"}}
	result := r.handleCommand(ctx, cmd)

	assert.False(t, result)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("restart was not called")
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
		preflight: preflight.NewMockPreflight(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	cmd := bus.Message{Type: bus.CommandStopAll}

	result := r.handleCommand(ctx, cmd)

	assert.True(t, result)
}

func Test_HandleCommand_InvalidData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		preflight: preflight.NewMockPreflight(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	cmd := bus.Message{Type: bus.CommandStopService, Data: "invalid"}

	result := r.handleCommand(ctx, cmd)

	assert.False(t, result)
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
		preflight: preflight.NewMockPreflight(ctrl),
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

	go func() {
		b.Publish(bus.Message{Type: bus.CommandStopAll})
	}()

	r.runServicePhase(ctx, cancel, sigChan, msgChan)
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
		preflight: preflight.NewMockPreflight(ctrl),
		service:   NewMockService(ctrl),
		pool:      NewMockWorkerPool(ctrl),
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx, cancel := context.WithCancel(context.Background())

	commandChan := make(chan bus.Message)
	sigChan := make(chan os.Signal, 1)

	go func() {
		cancel()
	}()

	r.runServicePhase(ctx, cancel, sigChan, commandChan)
}

func Test_RunServicePhase_CommandChannelClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		preflight: preflight.NewMockPreflight(ctrl),
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

	go func() {
		close(commandChan)
	}()

	r.runServicePhase(ctx, cancel, sigChan, commandChan)
}

func Test_StartTier_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}
	mockLog := logger.NewMockLogger(ctrl)

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", "platform").Return(nil)

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorkerPool.EXPECT().Release()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		preflight: preflight.NewMockPreflight(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()

	failedServices := r.startTier(ctx, "platform", []string{"api"})

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

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		preflight: preflight.NewMockPreflight(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()

	failedServices := r.startTier(ctx, "platform", []string{"api"})

	assert.Len(t, failedServices, 1)
	assert.Contains(t, failedServices, "api")
}

func Test_StartTier_ServiceStartupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", "platform").Return(errors.ErrServiceNotFound)

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorkerPool.EXPECT().Release()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		preflight: preflight.NewMockPreflight(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	failedServices := r.startTier(ctx, "platform", []string{"api"})

	assert.Len(t, failedServices, 1)
	assert.Contains(t, failedServices, "api")
}

func Test_RunStartupPhase_SignalDuringStartup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := process.NewMockProcess(ctrl)
	mockProcess.EXPECT().Name().Return("api").AnyTimes()

	startCalled := make(chan struct{})
	startCanProceed := make(chan struct{})

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", "platform").DoAndReturn(
		func(ctx context.Context, name, tier string) error {
			close(startCalled)
			<-startCanProceed

			return nil
		})
	mockService.EXPECT().Stop("api").AnyTimes()

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorkerPool.EXPECT().Release()

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockRegistry.EXPECT().SnapshotReverse().Return([]process.Process{mockProcess}).AnyTimes()
	mockRegistry.EXPECT().Detach("api").AnyTimes()
	mockRegistry.EXPECT().Wait().AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		preflight: preflight.NewMockPreflight(ctrl),
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
		<-startCalled

		sigChan <- syscall.SIGTERM

		close(startCanProceed)
	}()

	tiers := []discovery.Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, sigChan, commandChan)

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

	startCalled := make(chan struct{})
	startCanProceed := make(chan struct{})

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", "platform").DoAndReturn(
		func(ctx context.Context, name, tier string) error {
			close(startCalled)
			<-startCanProceed

			return ctx.Err()
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
		preflight: preflight.NewMockPreflight(ctrl),
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
		<-startCalled
		cancel()
		close(startCanProceed)
	}()

	tiers := []discovery.Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, sigChan, commandChan)

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

	startCalled := make(chan struct{})
	startCanProceed := make(chan struct{})

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", "platform").DoAndReturn(
		func(ctx context.Context, name, tier string) error {
			close(startCalled)
			<-startCanProceed

			return ctx.Err()
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
		preflight: preflight.NewMockPreflight(ctrl),
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
		<-startCalled
		close(commandChan)
		close(startCanProceed)
	}()

	tiers := []discovery.Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, sigChan, commandChan)

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

	startCalled := make(chan struct{})
	startCanProceed := make(chan struct{})

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", "platform").DoAndReturn(
		func(ctx context.Context, name, tier string) error {
			close(startCalled)
			<-startCanProceed

			return ctx.Err()
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
		preflight: preflight.NewMockPreflight(ctrl),
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
		<-startCalled

		commandChan <- bus.Message{Type: bus.CommandStopAll}

		close(startCanProceed)
	}()

	tiers := []discovery.Tier{{Name: "platform", Services: []string{"api"}}}
	err := r.runStartupPhase(ctx, cancel, tiers, sigChan, commandChan)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "startup interrupted")
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
		preflight: preflight.NewMockPreflight(ctrl),
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

	go func() {
		sigChan <- syscall.SIGTERM
	}()

	r.runServicePhase(ctx, cancel, sigChan, commandChan)
}

func Test_HandleMessage_WatchTriggered(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{
		Dir:   "api",
		Watch: &config.Watch{Include: []string{"*.go"}},
	}

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	restartCalled := make(chan struct{})
	releaseCalled := make(chan struct{})

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Restart(gomock.Any(), "api").Do(func(_ context.Context, _ string) {
		close(restartCalled)
	})

	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockWorkerPool.EXPECT().Acquire(gomock.Any()).Return(nil)
	mockWorkerPool.EXPECT().Release().Do(func() {
		close(releaseCalled)
	})

	r := &runner{
		cfg:       cfg,
		discovery: discovery.NewMockDiscovery(ctrl),
		preflight: preflight.NewMockPreflight(ctrl),
		service:   mockService,
		pool:      mockWorkerPool,
		registry:  registry.NewMockRegistry(ctrl),
		bus:       bus.NoOp(),
		log:       mockLog,
	}

	ctx := context.Background()
	msg := bus.Message{
		Type: bus.EventWatchTriggered,
		Data: bus.WatchTriggered{Service: "api", ChangedFiles: []string{"main.go"}},
	}

	result := r.handleMessage(ctx, msg)

	assert.False(t, result)

	select {
	case <-restartCalled:
	case <-time.After(time.Second):
		t.Fatal("Restart was not called")
	}

	select {
	case <-releaseCalled:
	case <-time.After(time.Second):
		t.Fatal("Release was not called")
	}
}

func Test_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}
	cfg.Services["web"] = &config.Service{Dir: "web"}

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("RUNNER").Return(componentLog)
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("test").Return([]discovery.Tier{
		{Name: "platform", Services: []string{"api", "web"}},
	}, nil)

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockPreflight := preflight.NewMockPreflight(ctrl)
	mockPreflight.EXPECT().Cleanup(gomock.Any()).Return(nil, nil)

	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockPreflight, mockService, mockWorkerPool, mockBus, mockServer, mockLog)

	err := r.Stop("test")

	assert.NoError(t, err)
}

func Test_Stop_ProfileNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("RUNNER").Return(componentLog)

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("nonexistent").Return(nil, errors.ErrProfileNotFound)

	mockRegistry := registry.NewMockRegistry(ctrl)

	mockPreflight := preflight.NewMockPreflight(ctrl)
	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockPreflight, mockService, mockWorkerPool, mockBus, mockServer, mockLog)

	err := r.Stop("nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve profile")
}

func Test_Stop_NoServices(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("RUNNER").Return(componentLog)
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()

	mockDiscovery := discovery.NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("empty").Return([]discovery.Tier{}, nil)

	mockRegistry := registry.NewMockRegistry(ctrl)
	mockPreflight := preflight.NewMockPreflight(ctrl)
	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockBus := bus.NoOp()
	mockServer := logs.NewMockServer(ctrl)

	r := NewRunner(cfg, mockDiscovery, mockRegistry, mockPreflight, mockService, mockWorkerPool, mockBus, mockServer, mockLog)

	err := r.Stop("empty")

	assert.NoError(t, err)
}

func Test_ResolveServiceDirs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	cfg.Services["api"] = &config.Service{Dir: "api"}
	cfg.Services["web"] = &config.Service{Dir: "/absolute/web"}

	mockLog := logger.NewMockLogger(ctrl)

	r := &runner{
		cfg: cfg,
		log: mockLog,
	}

	dirs := r.resolveServiceDirs([]string{"api", "web", "nonexistent"})

	assert.Len(t, dirs, 2)
	assert.Contains(t, dirs["api"], "api")
	assert.Equal(t, "/absolute/web", dirs["web"])
}
