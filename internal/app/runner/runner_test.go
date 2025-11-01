package runner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/errors"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewRunner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockDiscovery := NewMockDiscovery(ctrl)
	mockService := NewMockService(ctrl)
	cfg := &config.Config{}

	r := NewRunner(cfg, mockDiscovery, mockService, mockLogger)
	assert.NotNil(t, r)

	instance, ok := r.(*runner)
	assert.True(t, ok)
	assert.Equal(t, cfg, instance.cfg)
	assert.Equal(t, mockLogger, instance.log)
	assert.Equal(t, mockDiscovery, instance.discovery)
	assert.Equal(t, mockService, instance.service)
}

func Test_Run_ProfileNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	mockDiscovery := NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("nonexistent").Return(nil, errors.ErrProfileNotFound)

	mockService := NewMockService(ctrl)

	cfg := &config.Config{
		Services: map[string]*config.Service{},
	}

	r := NewRunner(cfg, mockDiscovery, mockService, mockLogger)

	ctx := context.Background()
	err := r.Run(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve profile")
}

func Test_Run_ServiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	mockDiscovery := NewMockDiscovery(ctrl)
	mockDiscovery.EXPECT().Resolve("test").Return(nil, errors.ErrServiceNotFound)

	mockService := NewMockService(ctrl)

	cfg := &config.Config{
		Services: map[string]*config.Service{},
	}

	r := NewRunner(cfg, mockDiscovery, mockService, mockLogger)

	ctx := context.Background()
	err := r.Run(ctx, "test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve profile")
}

func Test_Run_SuccessfulStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

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

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil)
	mockService.EXPECT().Stop(mockProcess).Return(nil)

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	r := NewRunner(cfg, mockDiscovery, mockService, mockLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.Run(ctx, "test")
	assert.NoError(t, err)
}

func Test_StartServiceWithRetry_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := NewMockProcess(ctrl)
	readyChan := make(chan error, 1)
	close(readyChan)
	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil)

	mockDiscovery := NewMockDiscovery(ctrl)

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: "api"},
		},
	}

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		log:       mockLogger,
	}

	ctx := context.Background()
	proc, err := r.startServiceWithRetry(ctx, "api", cfg.Services["api"])

	require.NoError(t, err)
	assert.NotNil(t, proc)
}

func Test_StartServiceWithRetry_ContextCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	mockProcess := NewMockProcess(ctrl)
	readyChan := make(chan error)
	mockProcess.EXPECT().Ready().Return(readyChan).AnyTimes()

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Start(gomock.Any(), "api", gomock.Any()).Return(mockProcess, nil)
	mockService.EXPECT().Stop(mockProcess).Return(nil)

	mockDiscovery := NewMockDiscovery(ctrl)

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

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		log:       mockLogger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	proc, err := r.startServiceWithRetry(ctx, "api", cfg.Services["api"])

	assert.Error(t, err)
	assert.Nil(t, proc)
	assert.Equal(t, context.Canceled, err)
}

func Test_StopAllProcesses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	mockProcess1 := NewMockProcess(ctrl)
	mockProcess2 := NewMockProcess(ctrl)

	mockService := NewMockService(ctrl)
	mockService.EXPECT().Stop(mockProcess2).Return(nil)
	mockService.EXPECT().Stop(mockProcess1).Return(nil)

	mockDiscovery := NewMockDiscovery(ctrl)

	cfg := &config.Config{}

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		log:       mockLogger,
	}

	processes := []Process{mockProcess1, mockProcess2}
	r.stopAllProcesses(processes)
}

func Test_StopAllProcesses_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockService := NewMockService(ctrl)
	mockDiscovery := NewMockDiscovery(ctrl)

	cfg := &config.Config{}

	r := &runner{
		cfg:       cfg,
		discovery: mockDiscovery,
		service:   mockService,
		log:       mockLogger,
	}

	processes := []Process{}
	r.stopAllProcesses(processes)
}
