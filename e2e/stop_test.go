package e2e

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Stop_StopsTheRunningInstance(t *testing.T) {
	runner := NewRunner(t, "testdata/default-tier")
	defer runner.Stop()

	require.NoError(t, runner.Start("default"))
	require.NoError(t, runner.WaitForRunning(15*time.Second))

	authPID, err := runner.ServicePID("auth-api", 10*time.Second)
	require.NoError(t, err)

	userPID, err := runner.ServicePID("user-api", 10*time.Second)
	require.NoError(t, err)

	result := RunOnce(t, "testdata/default-tier", "stop")

	assert.Equal(t, 0, result.ExitCode)
	assert.Regexp(t, `Stopping fuku · pid \d+ · profile default \.\.\. stopped in \d+\.\ds`, result.Stdout)

	require.NoError(t, WaitForGroupExit(authPID, 10*time.Second))
	require.NoError(t, WaitForGroupExit(userPID, 10*time.Second))

	output := runner.Output()

	assert.Contains(t, output, "phase=stopping", "stop shuts the instance down gracefully")
	assert.NotContains(t, output, "exited unexpectedly", "the instance stops its services, nothing kills them under it")
}

func Test_Stop_CleansUpOrphans(t *testing.T) {
	runner := NewRunner(t, "testdata/default-tier")
	defer runner.Stop()

	require.NoError(t, runner.Start("default"))
	require.NoError(t, runner.WaitForRunning(15*time.Second))
	require.NoError(t, runner.WaitForLogCount("Service ready", 2, 15*time.Second))

	authPID, err := runner.ServicePID("auth-api", 10*time.Second)
	require.NoError(t, err)

	userPID, err := runner.ServicePID("user-api", 10*time.Second)
	require.NoError(t, err)

	defer syscall.Kill(-authPID, syscall.SIGKILL)
	defer syscall.Kill(-userPID, syscall.SIGKILL)
	defer os.Remove(SocketPath(t, "testdata/default-tier"))

	runner.cmd.Process.Kill()
	runner.cmd.Wait()

	result := RunOnce(t, "testdata/default-tier", "stop", "default")

	assert.Equal(t, 0, result.ExitCode)
	assert.Regexp(t, `preflight_kill .*service=auth-api`, result.Stdout)
	assert.Regexp(t, `preflight_kill .*service=user-api`, result.Stdout)

	require.NoError(t, WaitForGroupExit(authPID, 10*time.Second))
	require.NoError(t, WaitForGroupExit(userPID, 10*time.Second))
	assert.NoFileExists(t, SocketPath(t, "testdata/default-tier"), "stop removes the socket the killed instance left")
}

func Test_Stop_NothingRunning(t *testing.T) {
	result := RunOnce(t, "testdata/default-tier", "stop")

	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "preflight_complete")
	assert.Contains(t, result.Stdout, "killed=0")
	assert.NotContains(t, result.Stdout, "preflight_kill")
	assert.Empty(t, result.Stderr)
}

func Test_Stop_UnknownProfile(t *testing.T) {
	result := RunOnce(t, "testdata/default-tier", "stop", "no-such-profile")

	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stderr, "profile not found: no-such-profile")
	assert.NotContains(t, result.Stdout, "preflight_started")
}
