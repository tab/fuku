package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx/fxevent"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_LoadConfig(t *testing.T) {
	cfg, topology, err := loadConfig()
	if err != nil {
		t.Skip("config loading failed, likely no fuku.yaml file in expected location")
		return
	} else {
		assert.NoError(t, err)
	}

	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.Services)
	assert.NotNil(t, cfg.Profiles)
	assert.NotNil(t, topology)
}

func Test_CreateApp(t *testing.T) {
	topology := &config.Topology{
		Order:        []string{},
		TierServices: make(map[string][]string),
	}

	tests := []struct {
		name   string
		config *config.Config
		noUI   bool
	}{
		{
			name: "Creates app with info level logging and TUI",
			config: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.InfoLevel,
				},
			},
			noUI: false,
		},
		{
			name: "Creates app with debug level logging and no UI",
			config: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.DebugLevel,
				},
			},
			noUI: true,
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
			noUI: false,
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
			noUI: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createApp(tt.config, topology, tt.noUI)
			assert.NotNil(t, app)
		})
	}
}

func Test_HasNoUIFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{name: "No args returns false", args: []string{}, expected: false},
		{name: "Only --run flag returns false", args: []string{"--run=default"}, expected: false},
		{name: "--no-ui flag returns true", args: []string{"--no-ui"}, expected: true},
		{name: "--run and --no-ui returns true", args: []string{"--run=default", "--no-ui"}, expected: true},
		{name: "--no-ui and --run returns true", args: []string{"--no-ui", "--run=core"}, expected: true},
		{name: "Other flags return false", args: []string{"help", "version"}, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasNoUIFlag(tt.args)
			assert.Equal(t, tt.expected, result)
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
