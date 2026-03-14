package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxevent"

	"fuku/internal/app/cli"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_LoadConfig(t *testing.T) {
	cfg, topology, err := loadConfig("")
	require.NoError(t, err)

	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.Services)
	assert.NotNil(t, cfg.Profiles)
	assert.NotNil(t, topology)
}

func Test_LoadConfig_WithExplicitPath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := `version: 1
services:
  web:
    dir: ./web
`

	filePath := filepath.Join(dir, "custom.yaml")
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, topology, err := loadConfig(filePath)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotNil(t, topology)
	assert.Contains(t, cfg.Services, "web")
}

func Test_LoadConfig_AfterChangeToConfigDir(t *testing.T) {
	dir := t.TempDir()

	subdir := filepath.Join(dir, "project")
	require.NoError(t, os.MkdirAll(subdir, 0755))

	content := `version: 1
services:
  api:
    dir: ./api
`
	require.NoError(t, os.WriteFile(filepath.Join(subdir, "fuku.yaml"), []byte(content), 0644))

	t.Chdir(dir)

	cmd := &cli.Options{ConfigFile: "project/fuku.yaml"}
	require.NoError(t, cli.ChangeToConfigDir(cmd))

	cfg, topology, err := loadConfig(cmd.ConfigFile)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotNil(t, topology)
	assert.Contains(t, cfg.Services, "api")
}

func Test_LoadConfig_ExplicitPathNotFound(t *testing.T) {
	t.Chdir(t.TempDir())

	_, _, err := loadConfig("nonexistent.yaml")
	require.Error(t, err)
}

func Test_CreateAppWithoutConfig(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cli.Options
	}{
		{
			name: "version command",
			cmd:  &cli.Options{Type: cli.CommandVersion},
		},
		{
			name: "help command",
			cmd:  &cli.Options{Type: cli.CommandHelp},
		},
		{
			name: "init command",
			cmd:  &cli.Options{Type: cli.CommandInit},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createAppWithoutConfig(tt.cmd)
			assert.NotNil(t, app)
		})
	}
}

func Test_CreateApp(t *testing.T) {
	topology := &config.Topology{
		Order:        []string{},
		TierServices: make(map[string][]string),
	}

	tests := []struct {
		name string
		cfg  *config.Config
		cmd  *cli.Options
	}{
		{
			name: "Creates app with info level logging and TUI",
			cfg: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.InfoLevel,
				},
			},
			cmd: &cli.Options{
				Type:    cli.CommandRun,
				Profile: config.Default,
				NoUI:    false,
			},
		},
		{
			name: "Creates app with debug level logging and no UI",
			cfg: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.DebugLevel,
				},
			},
			cmd: &cli.Options{
				Type:    cli.CommandRun,
				Profile: config.Default,
				NoUI:    true,
			},
		},
		{
			name: "Creates app with error level logging",
			cfg: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.ErrorLevel,
				},
			},
			cmd: &cli.Options{
				Type:    cli.CommandRun,
				Profile: config.Default,
				NoUI:    false,
			},
		},
		{
			name: "Creates app with warn level logging and logs mode",
			cfg: &config.Config{
				Logging: struct {
					Level  string `yaml:"level"`
					Format string `yaml:"format"`
				}{
					Level: logger.WarnLevel,
				},
			},
			cmd: &cli.Options{
				Type:    cli.CommandLogs,
				Profile: "",
				NoUI:    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createApp(tt.cfg, topology, tt.cmd)
			assert.NotNil(t, app)
		})
	}
}

func Test_CreateFxLogger(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		expectedType   any
		expectedLogger any
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
