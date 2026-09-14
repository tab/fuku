package bootstrap

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/cli"
)

func Test_StandaloneCLI_Run(t *testing.T) {
	tests := []struct {
		name           string
		cmd            *cli.Options
		readonly       bool
		expectedExit   int
		outputContains string
	}{
		{
			name:           "version command",
			cmd:            &cli.Options{Type: cli.CommandVersion},
			expectedExit:   0,
			outputContains: "Version",
		},
		{
			name:           "help command",
			cmd:            &cli.Options{Type: cli.CommandHelp},
			expectedExit:   0,
			outputContains: "Usage:",
		},
		{
			name:           "init command",
			cmd:            &cli.Options{Type: cli.CommandInit},
			expectedExit:   0,
			outputContains: "Created",
		},
		{
			name:           "init command in a read-only directory",
			cmd:            &cli.Options{Type: cli.CommandInit},
			readonly:       true,
			expectedExit:   1,
			outputContains: "Error: failed to write",
		},
		{
			name:           "other commands are ignored",
			cmd:            &cli.Options{Type: cli.CommandRun},
			expectedExit:   0,
			outputContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			if tt.readonly {
				require.NoError(t, os.Chmod(dir, 0o555))
				t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			}

			oldStdout, oldStderr := os.Stdout, os.Stderr
			r, w, err := os.Pipe()
			require.NoError(t, err)

			os.Stdout, os.Stderr = w, w

			exitCode := NewStandaloneCLI(tt.cmd).Run()

			w.Close()

			os.Stdout, os.Stderr = oldStdout, oldStderr

			var buf bytes.Buffer

			_, _ = io.Copy(&buf, r)

			assert.Equal(t, tt.expectedExit, exitCode)
			assert.Contains(t, buf.String(), tt.outputContains)
		})
	}
}
