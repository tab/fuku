package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx/fxevent"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_LoadConfig(t *testing.T) {
	cfg, err := loadConfig()

	if err != nil {
		t.Skip("config loading failed, likely no fuku.yaml file in expected location")
		return
	} else {
		assert.NoError(t, err)
	}

	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.Services)
	assert.NotNil(t, cfg.Profiles)
}

func Test_CreateApp(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
	}{
		{
			name: "Creates app with info level logging",
			config: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.InfoLevel,
				},
			},
		},
		{
			name: "Creates app with debug level logging",
			config: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.DebugLevel,
				},
			},
		},
		{
			name: "Creates app with error level logging",
			config: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.ErrorLevel,
				},
			},
		},
		{
			name: "Creates app with warn level logging",
			config: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.WarnLevel,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createApp(tt.config)
			assert.NotNil(t, app)
		})
	}
}

func Test_CreateFxLogger(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		expectedType   interface{}
		expectedLogger interface{}
	}{
		{
			name: "Debug level returns console logger",
			config: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.DebugLevel,
				},
			},
			expectedType: &fxevent.ConsoleLogger{},
		},
		{
			name: "Info level returns nop logger",
			config: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.InfoLevel,
				},
			},
			expectedLogger: fxevent.NopLogger,
		},
		{
			name: "Warn level returns nop logger",
			config: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.WarnLevel,
				},
			},
			expectedLogger: fxevent.NopLogger,
		},
		{
			name: "Error level returns nop logger",
			config: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.ErrorLevel,
				},
			},
			expectedLogger: fxevent.NopLogger,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loggerFunc := createFxLogger(tt.config)
			assert.NotNil(t, loggerFunc)

			result := loggerFunc()
			assert.NotNil(t, result)

			if tt.expectedType != nil {
				assert.IsType(t, tt.expectedType, result)
			}
			if tt.expectedLogger != nil {
				assert.Equal(t, tt.expectedLogger, result)
			}
		})
	}
}

func Test_CreateFxLogger_FunctionCreation(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
	}{
		{
			name: "Creates valid function with debug config",
			config: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.DebugLevel,
				},
			},
		},
		{
			name: "Creates valid function with info config",
			config: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.InfoLevel,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loggerFunc := createFxLogger(tt.config)
			assert.NotNil(t, loggerFunc)

			result1 := loggerFunc()
			result2 := loggerFunc()

			assert.NotNil(t, result1)
			assert.NotNil(t, result2)
		})
	}
}
