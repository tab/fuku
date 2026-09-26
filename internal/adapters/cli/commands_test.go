package cli

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/model"
)

// Flag arguments repeated across the parser tests
const (
	tailArg     = "--" + string(FlagTail)
	noUIArg     = "--" + string(FlagNoUI)
	noFollowArg = "--" + string(FlagNoFollow)
)

func Test_Parse(t *testing.T) {
	tail := 100

	tests := []struct {
		name               string
		args               []string
		expectedType       CommandType
		expectedProfile    string
		expectedServices   []string
		expectedTail       *int
		expectedNoUI       bool
		expectedNoFollow   bool
		expectedConfigFile string
		expectedFormat     Format
	}{
		{
			name:            "no args - default profile",
			args:            []string{},
			expectedType:    CommandRun,
			expectedProfile: model.ProfileDefault,
			expectedNoUI:    false,
		},
		{
			name:            "run command without profile",
			args:            []string{"run"},
			expectedType:    CommandRun,
			expectedProfile: model.ProfileDefault,
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
			args:            []string{"--run", "core", noUIArg},
			expectedType:    CommandRun,
			expectedProfile: "core",
			expectedNoUI:    true,
		},
		{
			name:            "--no-ui flag before run command",
			args:            []string{noUIArg, "run", "core"},
			expectedType:    CommandRun,
			expectedProfile: "core",
			expectedNoUI:    true,
		},
		{
			name:            "--no-ui flag after run command",
			args:            []string{"run", "core", noUIArg},
			expectedType:    CommandRun,
			expectedProfile: "core",
			expectedNoUI:    true,
		},
		{
			name:            "--no-ui flag with no command",
			args:            []string{noUIArg},
			expectedType:    CommandRun,
			expectedProfile: model.ProfileDefault,
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
			name:             "logs command with --tail",
			args:             []string{"logs", "api", tailArg, "100"},
			expectedType:     CommandLogs,
			expectedProfile:  "",
			expectedServices: []string{"api"},
			expectedTail:     &tail,
		},
		{
			name:             "logs command with --no-follow",
			args:             []string{"logs", "api", noFollowArg},
			expectedType:     CommandLogs,
			expectedProfile:  "",
			expectedServices: []string{"api"},
			expectedNoFollow: true,
		},
		{
			name:             "logs bounded read with service names first",
			args:             []string{"logs", "api", "--profile", "core", noUIArg, tailArg, "100", noFollowArg},
			expectedType:     CommandLogs,
			expectedProfile:  "core",
			expectedServices: []string{"api"},
			expectedTail:     &tail,
			expectedNoUI:     true,
			expectedNoFollow: true,
		},
		{
			name:             "logs bounded read with flags before service names",
			args:             []string{noUIArg, "logs", tailArg, "100", noFollowArg, "--profile", "core", "api"},
			expectedType:     CommandLogs,
			expectedProfile:  "core",
			expectedServices: []string{"api"},
			expectedTail:     &tail,
			expectedNoUI:     true,
			expectedNoFollow: true,
		},
		{
			name:             "logs bounded read with flags between service names",
			args:             []string{"logs", "api", tailArg, "100", "--profile", "core", "db", noFollowArg, noUIArg},
			expectedType:     CommandLogs,
			expectedProfile:  "core",
			expectedServices: []string{"api", "db"},
			expectedTail:     &tail,
			expectedNoUI:     true,
			expectedNoFollow: true,
		},
		{
			name:             "logs with --no-ui after the subcommand",
			args:             []string{"logs", "api", noUIArg},
			expectedType:     CommandLogs,
			expectedProfile:  "",
			expectedServices: []string{"api"},
			expectedNoUI:     true,
		},
		{
			name:            "stop command without profile",
			args:            []string{"stop"},
			expectedType:    CommandStop,
			expectedProfile: model.ProfileDefault,
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
			expectedProfile: model.ProfileDefault,
			expectedNoUI:    false,
		},
		{
			name:            "init alias i",
			args:            []string{"i"},
			expectedType:    CommandInit,
			expectedProfile: model.ProfileDefault,
			expectedNoUI:    false,
		},
		{
			name:            "--init flag",
			args:            []string{"--init"},
			expectedType:    CommandInit,
			expectedProfile: model.ProfileDefault,
			expectedNoUI:    false,
		},
		{
			name:            "-i flag",
			args:            []string{"-i"},
			expectedType:    CommandInit,
			expectedProfile: model.ProfileDefault,
			expectedNoUI:    false,
		},
		{
			name:            "version command",
			args:            []string{"version"},
			expectedType:    CommandVersion,
			expectedProfile: model.ProfileDefault,
			expectedNoUI:    false,
		},
		{
			name:            "--version flag",
			args:            []string{"--version"},
			expectedType:    CommandVersion,
			expectedProfile: model.ProfileDefault,
			expectedNoUI:    false,
		},
		{
			name:            "-v flag",
			args:            []string{"-v"},
			expectedType:    CommandVersion,
			expectedProfile: model.ProfileDefault,
			expectedNoUI:    false,
		},
		{
			name:            "help command",
			args:            []string{"help"},
			expectedType:    CommandHelp,
			expectedProfile: model.ProfileDefault,
			expectedNoUI:    false,
		},
		{
			name:            "--help flag",
			args:            []string{"--help"},
			expectedType:    CommandHelp,
			expectedProfile: model.ProfileDefault,
			expectedNoUI:    false,
		},
		{
			name:            "-h flag",
			args:            []string{"-h"},
			expectedType:    CommandHelp,
			expectedProfile: model.ProfileDefault,
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
			expectedProfile:    model.ProfileDefault,
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
		{
			name:            "doctor command",
			args:            []string{"doctor"},
			expectedType:    CommandDoctor,
			expectedProfile: model.ProfileDefault,
			expectedFormat:  FormatText,
		},
		{
			name:            "doctor command with profile",
			args:            []string{"doctor", "core"},
			expectedType:    CommandDoctor,
			expectedProfile: "core",
			expectedFormat:  FormatText,
		},
		{
			name:            "doctor --summary",
			args:            []string{"doctor", "--summary"},
			expectedType:    CommandDoctor,
			expectedProfile: model.ProfileDefault,
			expectedFormat:  FormatSummary,
		},
		{
			name:            "doctor --json",
			args:            []string{"doctor", "--json"},
			expectedType:    CommandDoctor,
			expectedProfile: model.ProfileDefault,
			expectedFormat:  FormatJSON,
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
			assert.Equal(t, tt.expectedTail, result.Tail)
			assert.Equal(t, tt.expectedNoUI, result.NoUI)
			assert.Equal(t, tt.expectedNoFollow, result.NoFollow)
			assert.Equal(t, tt.expectedConfigFile, result.ConfigFile)
			assert.Equal(t, tt.expectedFormat, result.DoctorFormat)
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

func Test_Parse_ConflictingFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "--run with --logs",
			args: []string{"--run", "core", "--logs"},
		},
		{
			name: "-v with -i",
			args: []string{"-v", "-i"},
		},
		{
			name: "--stop with --run",
			args: []string{"--stop", "core", "--run", "core"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.args)

			require.ErrorContains(t, err, "none of the others can be")
			assert.Nil(t, result)
		})
	}
}

func Test_Parse_InvalidTail(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "explicit zero",
			args: []string{"logs", tailArg, "0"},
		},
		{
			name: "negative value",
			args: []string{"logs", tailArg, "-1"},
		},
		{
			name: "negative value with service names",
			args: []string{"logs", "api", tailArg, "-5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.args)

			require.ErrorIs(t, err, ErrInvalidTail)
			assert.Nil(t, result)
		})
	}
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

			require.ErrorIs(t, err, ErrConfigFlagNotSupported)
			assert.Nil(t, result)
		})
	}
}

func Test_tailValue_String(t *testing.T) {
	five := 5

	tests := []struct {
		name     string
		tail     *int
		expected string
	}{
		{
			name:     "omitted flag",
			tail:     nil,
			expected: "",
		},
		{
			name:     "provided flag",
			tail:     &five,
			expected: "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := &tailValue{target: &tt.tail}

			assert.Equal(t, tt.expected, value.String())
		})
	}
}

func Test_tailValue_Set(t *testing.T) {
	hundred := 100

	tests := []struct {
		name          string
		raw           string
		expected      *int
		expectedError error
	}{
		{
			name:     "positive value",
			raw:      "100",
			expected: &hundred,
		},
		{
			name:          "zero",
			raw:           "0",
			expectedError: ErrInvalidTail,
		},
		{
			name:          "negative value",
			raw:           "-1",
			expectedError: ErrInvalidTail,
		},
		{
			name:          "not a number",
			raw:           "many",
			expectedError: strconv.ErrSyntax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tail *int

			err := (&tailValue{target: &tail}).Set(tt.raw)

			require.ErrorIs(t, err, tt.expectedError)
			assert.Equal(t, tt.expected, tail)
		})
	}
}

func Test_tailValue_Type(t *testing.T) {
	var tail *int

	assert.Equal(t, "int", (&tailValue{target: &tail}).Type())
}
