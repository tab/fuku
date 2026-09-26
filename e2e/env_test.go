package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Env_ChildInheritsEnvFiles(t *testing.T) {
	tests := []struct {
		name     string
		goEnv    string
		expected string
	}{
		{
			name:     "GO_ENV file wins over .env",
			goEnv:    "e2e",
			expected: "file=env goenv=goenv process=process",
		},
		{
			name:     "without GO_ENV only .env loads",
			goEnv:    "",
			expected: "file=env goenv=env process=process",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GO_ENV", tt.goEnv)
			t.Setenv("E2E_PROCESS_WINS", "process")

			runner := NewRunner(t, "testdata/env")
			defer runner.Stop()

			require.NoError(t, runner.Start("default"))
			require.NoError(t, runner.WaitForRunning(15*time.Second))
			require.NoError(t, runner.WaitForLog("goenv=", 10*time.Second))

			assert.Contains(t, runner.Output(), tt.expected)
		})
	}
}
