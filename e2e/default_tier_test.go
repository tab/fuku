package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_DefaultTier_StartsConcurrently(t *testing.T) {
	runner := NewRunner(t, "testdata/default-tier")
	defer runner.Stop()

	err := runner.Start("default")
	require.NoError(t, err)

	err = runner.WaitForServiceStarted("auth-api", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForServiceStarted("user-api", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForTierReady("default", 15*time.Second)
	require.NoError(t, err)

	err = runner.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	output := runner.Output()

	// Profile events
	assert.Contains(t, output, "profile_resolved {profile: default}")
	assert.Contains(t, output, "Starting services in profile 'default': [auth-api user-api]")

	// Tier events
	assert.Contains(t, output, "tier_starting {tier: default, 1/1}")
	assert.Contains(t, output, "tier_ready {name: default}")

	// Service events
	assert.Contains(t, output, "service_starting {service: auth-api")
	assert.Contains(t, output, "service_ready {service: auth-api")
	assert.Contains(t, output, "service_starting {service: user-api")
	assert.Contains(t, output, "service_ready {service: user-api")
}

func Test_DefaultTier_GracefulShutdown(t *testing.T) {
	runner := NewRunner(t, "testdata/default-tier")
	defer runner.Stop()

	err := runner.Start("default")
	require.NoError(t, err)

	err = runner.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	err = runner.Stop()
	require.NoError(t, err)

	output := runner.Output()

	// Shutdown
	assert.Contains(t, output, "signal {signal: terminated}")
	assert.Contains(t, output, "Received signal terminated, shutting down services")
}

func Test_DefaultTier_LogsCommand(t *testing.T) {
	runner := NewRunner(t, "testdata/default-tier")
	defer runner.Stop()

	err := runner.Start("default")
	require.NoError(t, err)

	err = runner.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	logsRunner := NewLogsRunner(t, "testdata/default-tier")
	defer logsRunner.Stop()

	err = logsRunner.Start("default")
	require.NoError(t, err)

	err = logsRunner.WaitForLog("ctrl+c", 5*time.Second)
	require.NoError(t, err)

	output := logsRunner.Output()

	assert.Contains(t, output, "profile:")
	assert.Contains(t, output, "default")
	assert.Contains(t, output, "services:")
	assert.Contains(t, output, "2 running")
}
