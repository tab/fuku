package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/errors"
	"fuku/internal/config"
)

func Test_CLI_Run(t *testing.T) {
	tests := []struct {
		name           string
		cmd            *Options
		expectedExit   int
		outputContains string
	}{
		{
			name:           "version command",
			cmd:            &Options{Type: CommandVersion},
			expectedExit:   0,
			outputContains: "Version",
		},
		{
			name:           "help command",
			cmd:            &Options{Type: CommandHelp},
			expectedExit:   0,
			outputContains: "Usage:",
		},
		{
			name:           "init command",
			cmd:            &Options{Type: CommandInit},
			expectedExit:   0,
			outputContains: "Created",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cmd.Type == CommandInit {
				t.Chdir(t.TempDir())
			}

			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)

			os.Stdout = w

			app := NewCLI(tt.cmd)
			exitCode := app.Run()

			w.Close()

			os.Stdout = oldStdout

			var buf bytes.Buffer

			_, _ = io.Copy(&buf, r)

			assert.Equal(t, tt.expectedExit, exitCode)
			assert.Contains(t, buf.String(), tt.outputContains)
		})
	}
}

func Test_GenerateConfigFile(t *testing.T) {
	tests := []struct {
		name           string
		existingFile   string
		readOnly       bool
		expectedExit   int
		expectedError  bool
		outputContains string
		assertFn       func(t *testing.T)
	}{
		{
			name:           "fuku.yaml already exists",
			existingFile:   config.ConfigFile,
			expectedExit:   0,
			outputContains: "fuku.yaml already exists",
			assertFn: func(t *testing.T) {
				content, err := os.ReadFile(config.ConfigFile)
				require.NoError(t, err)
				assert.Equal(t, "existing", string(content))
			},
		},
		{
			name:           "fuku.yml already exists",
			existingFile:   config.ConfigFileAlt,
			expectedExit:   0,
			outputContains: "fuku.yml already exists",
			assertFn: func(t *testing.T) {
				content, err := os.ReadFile(config.ConfigFileAlt)
				require.NoError(t, err)
				assert.Equal(t, "existing", string(content))
			},
		},
		{
			name:           "created successfully",
			expectedExit:   0,
			outputContains: "Created",
			assertFn: func(t *testing.T) {
				content, err := os.ReadFile(config.ConfigFile)
				require.NoError(t, err)
				assert.Contains(t, string(content), "version: 1")
				assert.Contains(t, string(content), "services:")
			},
		},
		{
			name:          "write error",
			readOnly:      true,
			expectedExit:  1,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			if tt.existingFile != "" {
				require.NoError(t, os.WriteFile(tt.existingFile, []byte("existing"), 0600))
			}

			if tt.readOnly {
				require.NoError(t, os.Chmod(dir, 0444))
				defer os.Chmod(dir, 0755)
			}

			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)

			os.Stdout = w

			exitCode, err := GenerateConfigFile()

			w.Close()

			os.Stdout = oldStdout

			var buf bytes.Buffer

			_, _ = io.Copy(&buf, r)

			assert.Equal(t, tt.expectedExit, exitCode)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Contains(t, buf.String(), tt.outputContains)
			}

			if tt.assertFn != nil {
				tt.assertFn(t)
			}
		})
	}
}

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
			expectedErr: errors.ErrFailedToReadConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startDir := t.TempDir()
			t.Chdir(startDir)

			cmd, expectedDir := tt.setup(t, startDir)
			err := ChangeToConfigDir(cmd)

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)

				return
			}

			require.NoError(t, err)
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
