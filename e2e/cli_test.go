package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_CLI_Version(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{name: "subcommand", arg: "version"},
		{name: "long flag", arg: "--version"},
		{name: "short flag", arg: "-v"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RunOnce(t, t.TempDir(), tt.arg)

			assert.Equal(t, 0, result.ExitCode)
			assert.Regexp(t, `^Version: \S+\n$`, result.Stdout)
			assert.Empty(t, result.Stderr)
		})
	}
}

func Test_CLI_Help(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{name: "subcommand", arg: "help"},
		{name: "long flag", arg: "--help"},
		{name: "short flag", arg: "-h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RunOnce(t, t.TempDir(), tt.arg)

			assert.Equal(t, 0, result.ExitCode)
			assert.Contains(t, result.Stdout, "Usage:")
			assert.Contains(t, result.Stdout, "fuku run <profile> --no-ui")
			assert.Empty(t, result.Stderr)
		})
	}
}

func Test_CLI_InvalidArguments(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "unknown command",
			args:     []string{"bogus"},
			expected: `Error: unknown command "bogus" for "fuku"`,
		},
		{
			name:     "unknown flag",
			args:     []string{"--bogus"},
			expected: "Error: unknown flag: --bogus",
		},
		{
			name:     "too many arguments",
			args:     []string{"run", "one", "two"},
			expected: "Error: accepts at most 1 arg(s), received 2",
		},
		{
			name:     "exclusive flags",
			args:     []string{"--version", "--run", "default"},
			expected: "if any flags in the group",
		},
		{
			name:     "config flag on a standalone command",
			args:     []string{"--config", "fuku.yaml", "init"},
			expected: "Error: --config flag is not supported for this command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			result := RunOnce(t, dir, tt.args...)

			assert.Equal(t, 1, result.ExitCode)
			assert.Contains(t, result.Stderr, tt.expected)
			assert.Empty(t, result.Stdout)
			assert.NoFileExists(t, filepath.Join(dir, "fuku.yaml"))
		})
	}
}

func Test_CLI_InitCreatesTemplate(t *testing.T) {
	dir := t.TempDir()

	result := RunOnce(t, dir, "init")

	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "Created fuku.yaml\n", result.Stdout)
	assert.Empty(t, result.Stderr)

	content, err := os.ReadFile(filepath.Join(dir, "fuku.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "version: 1")

	doctor := RunOnce(t, dir, "doctor", "--summary")

	assert.Equal(t, 0, doctor.ExitCode, "the template must load and validate")
	assert.Contains(t, doctor.Stdout, "schema ok")
}

func Test_CLI_InitKeepsExistingConfig(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "fuku.yaml", file: "fuku.yaml"},
		{name: "fuku.yml", file: "fuku.yml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.file)

			require.NoError(t, os.WriteFile(path, []byte("existing"), 0o600))

			result := RunOnce(t, dir, "init")

			assert.Equal(t, 0, result.ExitCode)
			assert.Equal(t, tt.file+" already exists\n", result.Stdout)

			content, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, "existing", string(content))

			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			assert.Len(t, entries, 1, "init must not write a second config beside the existing one")
		})
	}
}
