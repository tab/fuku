package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Logs_BoundedRead(t *testing.T) {
	runner := NewRunner(t, "testdata/default-tier")
	defer runner.Stop()

	require.NoError(t, runner.Start("default"))
	require.NoError(t, runner.WaitForRunning(15*time.Second))
	require.NoError(t, runner.WaitForLogCount("Service ready", 2, 15*time.Second))

	result := RunOnce(t, "testdata/default-tier",
		"logs", "auth-api", "--profile", "default", "--no-ui", "--tail", "1", "--no-follow")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "Service ready")
	assert.NotContains(t, result.Stdout, "Starting service")
	assert.NotContains(t, result.Stdout, "showing:")
	assert.NotContains(t, result.Stdout, "ctrl+c")
}

func Test_Logs_NoFollowReplaysMatchingHistory(t *testing.T) {
	runner := NewRunner(t, "testdata/default-tier")
	defer runner.Stop()

	require.NoError(t, runner.Start("default"))
	require.NoError(t, runner.WaitForRunning(15*time.Second))
	require.NoError(t, runner.WaitForLogCount("Service ready", 2, 15*time.Second))

	result := RunOnce(t, "testdata/default-tier",
		"logs", "auth-api", "--profile", "default", "--no-ui", "--no-follow")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "Starting service")
	assert.Contains(t, result.Stdout, "Service ready")
}

func Test_Logs_InvalidTailFailsBeforeConnecting(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "explicit zero",
			args: []string{"logs", "--tail", "0"},
		},
		{
			name: "negative value",
			args: []string{"logs", "auth-api", "--tail", "-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RunOnce(t, "testdata/default-tier", tt.args...)

			assert.Equal(t, 1, result.ExitCode)
			assert.Contains(t, result.Stderr, "--tail must be greater than zero")
		})
	}
}
