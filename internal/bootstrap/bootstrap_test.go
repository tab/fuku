package bootstrap

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Run(t *testing.T) {
	untouched := func(*testing.T, string) {}

	tests := []struct {
		name           string
		before         func(t *testing.T, dir string)
		args           []string
		expectedExit   int
		outputContains string
	}{
		{
			name:           "parse error",
			before:         untouched,
			args:           []string{"--no-such-flag"},
			expectedExit:   1,
			outputContains: "Error: unknown flag",
		},
		{
			name:           "version",
			before:         untouched,
			args:           []string{"version"},
			expectedExit:   0,
			outputContains: "Version",
		},
		{
			name:           "help",
			before:         untouched,
			args:           []string{"help"},
			expectedExit:   0,
			outputContains: "Usage:",
		},
		{
			name:           "init",
			before:         untouched,
			args:           []string{"init"},
			expectedExit:   0,
			outputContains: "Created",
		},
		{
			name: "init in a read-only directory",
			before: func(t *testing.T, dir string) {
				require.NoError(t, os.Chmod(dir, 0o555))
				t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			},
			args:           []string{"init"},
			expectedExit:   1,
			outputContains: "Error: failed to write",
		},
		{
			name:           "doctor without config",
			before:         untouched,
			args:           []string{"doctor", "--summary"},
			expectedExit:   2,
			outputContains: "config",
		},
		{
			name:           "doctor with a missing config directory",
			before:         untouched,
			args:           []string{"--config", "missing/fuku.yaml", "doctor"},
			expectedExit:   1,
			outputContains: "Error: failed to read config file",
		},
		{
			name: "doctor in a deleted working directory",
			before: func(t *testing.T, dir string) {
				require.NoError(t, os.RemoveAll(dir))
			},
			args:           []string{"doctor", "--summary"},
			expectedExit:   1,
			outputContains: "Error:",
		},
		{
			name:           "run with a missing config directory",
			before:         untouched,
			args:           []string{"--config", "missing/fuku.yaml", "run"},
			expectedExit:   1,
			outputContains: "Error: failed to read config file",
		},
		{
			name:           "run with a missing config file",
			before:         untouched,
			args:           []string{"--config", "missing.yaml", "run"},
			expectedExit:   1,
			outputContains: "Error: failed to read config file",
		},
		{
			name:           "run without services",
			before:         untouched,
			args:           []string{"run"},
			expectedExit:   1,
			outputContains: "Error: no services defined",
		},
		{
			name:           "stop without services",
			before:         untouched,
			args:           []string{"stop"},
			expectedExit:   1,
			outputContains: "Error: no services defined",
		},
		{
			name:           "logs without an instance",
			before:         untouched,
			args:           []string{"logs", "--no-ui", "--no-follow"},
			expectedExit:   1,
			outputContains: "Error: no fuku instance is running for project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			t.Setenv("SENTRY_DSN", "")
			t.Setenv("FUKU_UPDATER_DISABLED", "1")

			tt.before(t, dir)

			oldStdout, oldStderr := os.Stdout, os.Stderr
			r, w, err := os.Pipe()
			require.NoError(t, err)

			os.Stdout, os.Stderr = w, w

			exitCode := Run(tt.args, "")

			w.Close()

			os.Stdout, os.Stderr = oldStdout, oldStderr

			var buf bytes.Buffer

			_, _ = io.Copy(&buf, r)

			assert.Equal(t, tt.expectedExit, exitCode)
			assert.Contains(t, buf.String(), tt.outputContains)
		})
	}
}
