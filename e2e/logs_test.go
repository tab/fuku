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

func Test_Logs_ProjectScoped(t *testing.T) {
	tier := NewRunner(t, "testdata/tier")
	defer tier.Stop()

	defaultTier := NewRunner(t, "testdata/default-tier")
	defer defaultTier.Stop()

	require.NoError(t, tier.Start("default"))
	require.NoError(t, tier.WaitForRunning(30*time.Second))

	require.NoError(t, defaultTier.Start("default"))
	require.NoError(t, defaultTier.WaitForRunning(15*time.Second))
	require.NoError(t, defaultTier.WaitForLogCount("Service ready", 2, 15*time.Second))

	require.FileExists(t, SocketPath(t, "testdata/tier"))
	require.FileExists(t, SocketPath(t, "testdata/default-tier"))

	tierLogs := RunOnce(t, "testdata/tier", "logs", "--no-ui", "--no-follow")

	assert.Equal(t, 0, tierLogs.ExitCode)
	assert.Contains(t, tierLogs.Stdout, "postgres")
	assert.Contains(t, tierLogs.Stdout, "gateway")
	assert.NotContains(t, tierLogs.Stdout, "auth-api")
	assert.NotContains(t, tierLogs.Stdout, "user-api")

	defaultLogs := RunOnce(t, "testdata/default-tier", "logs", "--no-ui", "--no-follow")

	assert.Equal(t, 0, defaultLogs.ExitCode)
	assert.Contains(t, defaultLogs.Stdout, "auth-api")
	assert.Contains(t, defaultLogs.Stdout, "user-api")
	assert.NotContains(t, defaultLogs.Stdout, "postgres")
	assert.NotContains(t, defaultLogs.Stdout, "gateway")
}

func Test_Logs_ProfileMismatch(t *testing.T) {
	runner := NewRunner(t, "testdata/default-tier")
	defer runner.Stop()

	require.NoError(t, runner.Start("default"))
	require.NoError(t, runner.WaitForRunning(15*time.Second))

	result := RunOnce(t, "testdata/default-tier", "logs", "--profile", "core", "--no-ui", "--no-follow")

	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stderr, "'default'")
	assert.Contains(t, result.Stderr, "'core'")
	assert.NotContains(t, result.Stdout, "Service ready")
}

func Test_Logs_NoInstance(t *testing.T) {
	result := RunOnce(t, t.TempDir(), "logs")

	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stderr, "Error: no fuku instance is running for project")
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
