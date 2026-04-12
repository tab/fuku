package api

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/registry"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("API").Return(mockLog)

	cfg := config.DefaultConfig()
	cfg.Server.Listen = "127.0.0.1:9876"
	cfg.Server.Auth.Token = "test"

	s := NewServer(cfg, nil, nil, mockLog)

	assert.NotNil(t, s)
	assert.Equal(t, cfg, s.cfg)
}

func Test_Server_StartAndShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)
	mockStore := registry.NewMockStore(ctrl)
	mockLog := logger.NewMockLogger(ctrl)

	mockLog.EXPECT().WithComponent("API").Return(mockLog)
	mockLog.EXPECT().Info().Return(logger.NewLoggerWithOutput(config.DefaultConfig(), io.Discard).Info()).AnyTimes()

	mockBus.EXPECT().Publish(gomock.Any()).AnyTimes()
	mockStore.EXPECT().Phase().Return("").AnyTimes()

	cfg := config.DefaultConfig()
	cfg.Server.Listen = "127.0.0.1:0"

	s := NewServer(cfg, mockStore, mockBus, mockLog)
	s.Start()

	require.NotNil(t, s.httpServer)
	assert.NotEmpty(t, s.Address())
	assert.NotContains(t, s.Address(), ":0")

	resp, err := http.Get("http://" + s.Address() + "/api/v1/live")
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s.Shutdown(ctx)
}

func Test_Server_Shutdown_NilServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("API").Return(mockLog)

	s := NewServer(config.DefaultConfig(), nil, nil, mockLog)

	s.Shutdown(context.Background())
}

func Test_Server_Start_PortBusy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)
	mockStore := registry.NewMockStore(ctrl)
	mockLog := logger.NewMockLogger(ctrl)

	mockLog.EXPECT().WithComponent("API").Return(mockLog)
	mockLog.EXPECT().Warn().Return(logger.NewLoggerWithOutput(config.DefaultConfig(), io.Discard).Warn()).AnyTimes()

	cfg := config.DefaultConfig()
	cfg.Server.Listen = "127.0.0.1:1"

	s := NewServer(cfg, mockStore, mockBus, mockLog)
	s.Start()

	assert.Nil(t, s.httpServer)
}
