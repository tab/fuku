package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/errors"
	"fuku/internal/config"
)

func Test_Parse(t *testing.T) {
	tests := []struct {
		name               string
		args               []string
		expectedType       CommandType
		expectedProfile    string
		expectedServices   []string
		expectedNoUI       bool
		expectedConfigFile string
	}{
		{
			name:            "no args - default profile",
			args:            []string{},
			expectedType:    CommandRun,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:            "run command without profile",
			args:            []string{"run"},
			expectedType:    CommandRun,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:            "run command with profile",
			args:            []string{"run", "core"},
			expectedType:    CommandRun,
			expectedProfile: "core",
			expectedNoUI:    false,
		},
		{
			name:            "run alias r with profile",
			args:            []string{"r", "backend"},
			expectedType:    CommandRun,
			expectedProfile: "backend",
			expectedNoUI:    false,
		},
		{
			name:            "--run flag with profile",
			args:            []string{"--run", "backend"},
			expectedType:    CommandRun,
			expectedProfile: "backend",
			expectedNoUI:    false,
		},
		{
			name:            "-r flag with profile",
			args:            []string{"-r", "backend"},
			expectedType:    CommandRun,
			expectedProfile: "backend",
			expectedNoUI:    false,
		},
		{
			name:            "--run flag with --no-ui",
			args:            []string{"--run", "core", "--no-ui"},
			expectedType:    CommandRun,
			expectedProfile: "core",
			expectedNoUI:    true,
		},
		{
			name:            "--no-ui flag before run command",
			args:            []string{"--no-ui", "run", "core"},
			expectedType:    CommandRun,
			expectedProfile: "core",
			expectedNoUI:    true,
		},
		{
			name:            "--no-ui flag after run command",
			args:            []string{"run", "core", "--no-ui"},
			expectedType:    CommandRun,
			expectedProfile: "core",
			expectedNoUI:    true,
		},
		{
			name:            "--no-ui flag with no command",
			args:            []string{"--no-ui"},
			expectedType:    CommandRun,
			expectedProfile: config.Default,
			expectedNoUI:    true,
		},
		{
			name:             "logs command without services",
			args:             []string{"logs"},
			expectedType:     CommandLogs,
			expectedProfile:  "",
			expectedServices: []string{},
			expectedNoUI:     false,
		},
		{
			name:             "logs command with services",
			args:             []string{"logs", "api", "db"},
			expectedType:     CommandLogs,
			expectedProfile:  "",
			expectedServices: []string{"api", "db"},
			expectedNoUI:     false,
		},
		{
			name:             "logs command with --profile",
			args:             []string{"logs", "--profile", "core", "api"},
			expectedType:     CommandLogs,
			expectedProfile:  "core",
			expectedServices: []string{"api"},
			expectedNoUI:     false,
		},
		{
			name:             "logs alias l with services",
			args:             []string{"l", "api", "db"},
			expectedType:     CommandLogs,
			expectedProfile:  "",
			expectedServices: []string{"api", "db"},
			expectedNoUI:     false,
		},
		{
			name:             "--logs flag",
			args:             []string{"--logs"},
			expectedType:     CommandLogs,
			expectedProfile:  "",
			expectedServices: []string{},
			expectedNoUI:     false,
		},
		{
			name:             "-l flag",
			args:             []string{"-l"},
			expectedType:     CommandLogs,
			expectedProfile:  "",
			expectedServices: []string{},
			expectedNoUI:     false,
		},
		{
			name:            "stop command without profile",
			args:            []string{"stop"},
			expectedType:    CommandStop,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:            "stop command with profile",
			args:            []string{"stop", "core"},
			expectedType:    CommandStop,
			expectedProfile: "core",
			expectedNoUI:    false,
		},
		{
			name:            "stop alias s with profile",
			args:            []string{"s", "backend"},
			expectedType:    CommandStop,
			expectedProfile: "backend",
			expectedNoUI:    false,
		},
		{
			name:            "--stop flag with profile",
			args:            []string{"--stop", "backend"},
			expectedType:    CommandStop,
			expectedProfile: "backend",
			expectedNoUI:    false,
		},
		{
			name:            "-s flag with profile",
			args:            []string{"-s", "backend"},
			expectedType:    CommandStop,
			expectedProfile: "backend",
			expectedNoUI:    false,
		},
		{
			name:            "init command",
			args:            []string{"init"},
			expectedType:    CommandInit,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:            "init alias i",
			args:            []string{"i"},
			expectedType:    CommandInit,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:            "--init flag",
			args:            []string{"--init"},
			expectedType:    CommandInit,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:            "-i flag",
			args:            []string{"-i"},
			expectedType:    CommandInit,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:            "version command",
			args:            []string{"version"},
			expectedType:    CommandVersion,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:            "--version flag",
			args:            []string{"--version"},
			expectedType:    CommandVersion,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:            "-v flag",
			args:            []string{"-v"},
			expectedType:    CommandVersion,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:            "help command",
			args:            []string{"help"},
			expectedType:    CommandHelp,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:            "--help flag",
			args:            []string{"--help"},
			expectedType:    CommandHelp,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:            "-h flag",
			args:            []string{"-h"},
			expectedType:    CommandHelp,
			expectedProfile: config.Default,
			expectedNoUI:    false,
		},
		{
			name:               "--config with run command",
			args:               []string{"--config", "custom.yaml", "run", "core"},
			expectedType:       CommandRun,
			expectedProfile:    "core",
			expectedNoUI:       false,
			expectedConfigFile: "custom.yaml",
		},
		{
			name:               "-c shorthand with run command",
			args:               []string{"-c", "custom.yaml", "run"},
			expectedType:       CommandRun,
			expectedProfile:    config.Default,
			expectedNoUI:       false,
			expectedConfigFile: "custom.yaml",
		},
		{
			name:            "no --config flag",
			args:            []string{"run", "core"},
			expectedType:    CommandRun,
			expectedProfile: "core",
			expectedNoUI:    false,
		},
		{
			name:               "--config with logs command",
			args:               []string{"--config", "other.yaml", "logs"},
			expectedType:       CommandLogs,
			expectedProfile:    "",
			expectedServices:   []string{},
			expectedConfigFile: "other.yaml",
		},
		{
			name:               "--config with stop command",
			args:               []string{"-c", "other.yaml", "stop", "backend"},
			expectedType:       CommandStop,
			expectedProfile:    "backend",
			expectedNoUI:       false,
			expectedConfigFile: "other.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.args)

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedType, result.Type)
			assert.Equal(t, tt.expectedProfile, result.Profile)
			assert.Equal(t, tt.expectedServices, result.Services)
			assert.Equal(t, tt.expectedNoUI, result.NoUI)
			assert.Equal(t, tt.expectedConfigFile, result.ConfigFile)
		})
	}
}

func Test_CommandType_Standalone(t *testing.T) {
	tests := []struct {
		name     string
		cmd      CommandType
		expected bool
	}{
		{
			name:     "init is standalone",
			cmd:      CommandInit,
			expected: true,
		},
		{
			name:     "version is standalone",
			cmd:      CommandVersion,
			expected: true,
		},
		{
			name:     "help is standalone",
			cmd:      CommandHelp,
			expected: true,
		},
		{
			name:     "run is not standalone",
			cmd:      CommandRun,
			expected: false,
		},
		{
			name:     "stop is not standalone",
			cmd:      CommandStop,
			expected: false,
		},
		{
			name:     "logs is not standalone",
			cmd:      CommandLogs,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cmd.Standalone())
		})
	}
}

func Test_Parse_InvalidCommand(t *testing.T) {
	result, err := Parse([]string{"unknown"})
	require.Error(t, err)
	assert.Nil(t, result)
}

func Test_Parse_RunWithTooManyArgs(t *testing.T) {
	result, err := Parse([]string{"run", "profile1", "profile2"})
	require.Error(t, err)
	assert.Nil(t, result)
}

func Test_Parse_StopWithTooManyArgs(t *testing.T) {
	result, err := Parse([]string{"stop", "profile1", "profile2"})
	require.Error(t, err)
	assert.Nil(t, result)
}

func Test_Parse_ConfigFlagNotSupported(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "--config with init command",
			args: []string{"--config", "custom.yaml", "init"},
		},
		{
			name: "-c with init flag",
			args: []string{"-c", "custom.yaml", "-i"},
		},
		{
			name: "--config with version command",
			args: []string{"--config", "custom.yaml", "version"},
		},
		{
			name: "-c with version flag",
			args: []string{"-c", "custom.yaml", "-v"},
		},
		{
			name: "--config with help command",
			args: []string{"--config", "custom.yaml", "help"},
		},
		{
			name: "-c with help flag",
			args: []string{"-c", "custom.yaml", "-h"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.args)

			require.ErrorIs(t, err, errors.ErrConfigFlagNotSupported)
			assert.Nil(t, result)
		})
	}
}
