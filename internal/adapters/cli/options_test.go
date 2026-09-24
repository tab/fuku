package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/contracts"
)

func Test_ChangeToConfigDir(t *testing.T) {
	tests := []struct {
		name               string
		setup              func(t *testing.T, startDir string) (*Options, string)
		expectedConfigFile string
		expectedErr        error
	}{
		{
			name: "empty config file does nothing",
			setup: func(t *testing.T, startDir string) (*Options, string) {
				return &Options{}, startDir
			},
		},
		{
			name: "basename only does not chdir",
			setup: func(t *testing.T, startDir string) (*Options, string) {
				return &Options{ConfigFile: "fuku.yaml"}, startDir
			},
			expectedConfigFile: "fuku.yaml",
		},
		{
			name: "relative path with directory changes to subdirectory",
			setup: func(t *testing.T, startDir string) (*Options, string) {
				subdir := filepath.Join(startDir, "subdir")
				require.NoError(t, os.MkdirAll(subdir, 0755))

				return &Options{ConfigFile: "subdir/fuku.yaml"}, subdir
			},
			expectedConfigFile: "fuku.yaml",
		},
		{
			name: "absolute path changes to config parent directory",
			setup: func(t *testing.T, startDir string) (*Options, string) {
				targetDir := t.TempDir()

				return &Options{ConfigFile: filepath.Join(targetDir, "fuku.yaml")}, targetDir
			},
			expectedConfigFile: "fuku.yaml",
		},
		{
			name: "missing parent directory returns config error",
			setup: func(t *testing.T, startDir string) (*Options, string) {
				return &Options{ConfigFile: "/tmp/does-not-exist-dir/fuku.yaml"}, startDir
			},
			expectedConfigFile: "/tmp/does-not-exist-dir/fuku.yaml",
			expectedErr:        contracts.ErrFailedToReadConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startDir := t.TempDir()
			t.Chdir(startDir)

			cmd, expectedDir := tt.setup(t, startDir)
			err := ChangeToConfigDir(cmd)

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedConfigFile, cmd.ConfigFile)

			rawCwd, err := os.Getwd()
			require.NoError(t, err)

			cwd, err := filepath.EvalSymlinks(rawCwd)
			require.NoError(t, err)

			expected, err := filepath.EvalSymlinks(expectedDir)
			require.NoError(t, err)

			assert.Equal(t, expected, cwd)
		})
	}
}

func Test_CommandType_standalone(t *testing.T) {
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
		{
			name:     "doctor is not standalone",
			cmd:      CommandDoctor,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cmd.standalone())
		})
	}
}

func Test_CommandType_RequiresServices(t *testing.T) {
	tests := []struct {
		name     string
		cmd      CommandType
		expected bool
	}{
		{
			name:     "run requires services",
			cmd:      CommandRun,
			expected: true,
		},
		{
			name:     "stop requires services",
			cmd:      CommandStop,
			expected: true,
		},
		{
			name:     "doctor does not require services",
			cmd:      CommandDoctor,
			expected: false,
		},
		{
			name:     "logs does not require services",
			cmd:      CommandLogs,
			expected: false,
		},
		{
			name:     "init does not require services",
			cmd:      CommandInit,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cmd.RequiresServices())
		})
	}
}
