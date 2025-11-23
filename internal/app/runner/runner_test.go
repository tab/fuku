package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
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
	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)

	mockEvent := runtime.NewNoOpEventBus()
	mockCommand := runtime.NewNoOpCommandBus()

	r := NewRunner(cfg, mockDiscovery, mockService, mockWorkerPool, mockLogger, mockEvent, mockCommand)
	assert.NotNil(t, r)

	instance, ok := r.(*runner)
	assert.True(t, ok)
	assert.Equal(t, cfg, instance.cfg)
	assert.Equal(t, mockLogger, instance.log)
	assert.Equal(t, mockDiscovery, instance.discovery)
	assert.Equal(t, mockService, instance.service)
	assert.Equal(t, mockWorkerPool, instance.pool)
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

	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockEvent := runtime.NewNoOpEventBus()
	mockCommand := runtime.NewNoOpCommandBus()

	r := NewRunner(cfg, mockDiscovery, mockService, mockWorkerPool, mockLogger, mockEvent, mockCommand)

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

	mockService := NewMockService(ctrl)
	mockWorkerPool := NewMockWorkerPool(ctrl)
	mockEvent := runtime.NewNoOpEventBus()
	mockCommand := runtime.NewNoOpCommandBus()

	r := NewRunner(cfg, mockDiscovery, mockService, mockWorkerPool, mockLogger, mockEvent, mockCommand)

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
	mockWorkerPool.EXPECT().Acquire().AnyTimes()
	mockWorkerPool.EXPECT().Release().AnyTimes()

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      mockWorkerPool,
		log:       mockLogger,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.Run(ctx, "test")
	assert.NoError(t, err)
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

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      NewWorkerPool(),
		log:       mockLogger,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
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

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      NewWorkerPool(),
		log:       mockLogger,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
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

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewWorkerPool(),
		log:       mockLogger,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(100 * time.Millisecond)
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

func Test_StopAllProcesses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}

	mockLogger := logger.NewMockLogger(ctrl)

	mockProcess1 := NewMockProcess(ctrl)
	mockProcess2 := NewMockProcess(ctrl)

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockProcess2).Return(nil)
	mockService.EXPECT().Stop(mockProcess1).Return(nil)

	mockDiscovery := NewMockDiscovery(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      NewWorkerPool(),
		log:       mockLogger,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
	}

	processes := []Process{mockProcess1, mockProcess2}
	r.stopAllProcesses(processes)
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
	mockService.EXPECT().Stop(mockProcess2).Return(nil).Times(1)
	mockService.EXPECT().Stop(mockProcess1).Return(nil).Times(1)

	r := &runner{
		cfg:       cfg,
		discovery: NewMockDiscovery(ctrl),
		service:   mockService,
		pool:      NewWorkerPool(),
		log:       mockLogger,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
	}

	processes := []Process{mockProcess1, mockProcess2}

	var processesMu sync.Mutex

	var wg sync.WaitGroup

	r.shutdown(processes, &processesMu, &wg)
}

func Test_StopAllProcesses_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}

	mockLogger := logger.NewMockLogger(ctrl)
	mockService := NewMockService(ctrl)
	mockDiscovery := NewMockDiscovery(ctrl)

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		pool:      NewWorkerPool(),
		log:       mockLogger,
		event:     runtime.NewNoOpEventBus(),
		command:   runtime.NewNoOpCommandBus(),
	}

	processes := []Process{}
	r.stopAllProcesses(processes)
}
