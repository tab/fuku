package e2e

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Readiness_TCPAndHTTPProbesPass(t *testing.T) {
	runner := NewRunner(t, "testdata/readiness")
	defer runner.Stop()

	require.NoError(t, runner.Start("healthy"))
	require.NoError(t, runner.WaitForRunning(30*time.Second))
	require.NoError(t, runner.WaitForLogCount("readiness_complete", 2, 10*time.Second))

	output := runner.Output()

	assert.Regexp(t, `readiness_complete .*service=tcp-api type=tcp`, output)
	assert.Regexp(t, `readiness_complete .*service=http-api type=http`, output)
	assert.Contains(t, output, "tier_ready")
	assert.NotContains(t, output, "service_failed")
}

func Test_Readiness_RetriesThenFails(t *testing.T) {
	runner := NewRunner(t, "testdata/readiness")
	defer runner.Stop()

	require.NoError(t, runner.Start("unhealthy"))
	require.NoError(t, runner.WaitForRunning(30*time.Second))
	require.NoError(t, runner.WaitForLogCount("service_failed", 2, 10*time.Second))

	output := runner.Output()

	assert.Regexp(t, `service_starting attempt=2 .*service=unreachable`, output)
	assert.Regexp(t, `service_starting attempt=2 .*service=crashing`, output)
	assert.NotContains(t, output, "attempt=3")
	assert.Regexp(t, `service_failed error="[^"]*timed out[^"]*" .*service=unreachable`, output)
	assert.Regexp(t, `service_failed error="[^"]*process exited before readiness" .*service=crashing`, output)
	assert.NotContains(t, output, "tier_ready")

	failed := [2]any{float64(0), "failed"}
	bothFailed := func() bool {
		processes := serviceProcesses(t)

		return processes["unreachable"] == failed && processes["crashing"] == failed
	}

	assert.Eventually(t, bothFailed, 5*time.Second, 100*time.Millisecond, "the API must report both services failed")
}

func Test_Readiness_PortInUseFailsWithoutLaunch(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:19882")
	require.NoError(t, err)

	defer listener.Close()

	runner := NewRunner(t, "testdata/readiness")
	defer runner.Stop()

	require.NoError(t, runner.Start("unhealthy"))
	require.NoError(t, runner.WaitForRunning(30*time.Second))
	require.NoError(t, runner.WaitForLogCount("service_failed", 2, 10*time.Second))

	output := runner.Output()

	assert.Regexp(t, `service_failed error="[^"]*port already in use: 127.0.0.1:19882" .*service=unreachable`, output)
	assert.NotContains(t, output, "Started service 'unreachable'")
}
