package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx/fxevent"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_CreateApp(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logging.Level = logger.InfoLevel

	app := createApp(cfg)
	assert.NotNil(t, app)
}

func Test_CreateApp_WithDebugLogging(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logging.Level = logger.DebugLevel

	app := createApp(cfg)
	assert.NotNil(t, app)
}

func Test_CreateFxLogger_DebugLevel(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logging.Level = logger.DebugLevel

	loggerFunc := createFxLogger(cfg)()
	assert.IsType(t, &fxevent.ConsoleLogger{}, loggerFunc)
}

func Test_CreateFxLogger_NonDebugLevel(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logging.Level = logger.InfoLevel

	loggerFunc := createFxLogger(cfg)()
	assert.Equal(t, fxevent.NopLogger, loggerFunc)
}

func Test_CreateFxLogger_FunctionType(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logging.Level = logger.InfoLevel

	loggerFunc := createFxLogger(cfg)
	assert.NotNil(t, loggerFunc)

	result := loggerFunc()
	assert.NotNil(t, result)
}

func Test_LoadConfig(t *testing.T) {
	cfg, err := loadConfig()
	if err != nil {
		t.Skip("Config loading failed, likely no fuku.yaml file in expected location")
		return
	}

	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.Services)
	assert.NotNil(t, cfg.Scopes)
}
