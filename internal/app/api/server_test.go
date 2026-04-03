package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewServer_APIDisabled(t *testing.T) {
	cfg := config.DefaultConfig()

	s := NewServer(cfg, nil, nil, nil)

	_, ok := s.(*noOpServer)
	assert.True(t, ok)
	assert.NoError(t, s.Start(context.Background()))
	assert.NoError(t, s.Stop())
}

func Test_NewServer_APIEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("API").Return(mockLog)

	cfg := config.DefaultConfig()
	cfg.API = &config.APIConfig{
		Listen: "127.0.0.1:0",
		Auth:   config.AuthConfig{Token: "test"},
	}

	s := NewServer(cfg, nil, nil, mockLog)

	_, ok := s.(*server)
	assert.True(t, ok)
}
