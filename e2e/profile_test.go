package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Profile_RunsOnlyItsServices(t *testing.T) {
	runner := NewRunner(t, "testdata/readiness")
	defer runner.Stop()

	require.NoError(t, runner.Start("healthy"))
	require.NoError(t, runner.WaitForRunning(30*time.Second))

	output := runner.Output()

	assert.Contains(t, output, "profile_resolved profile=healthy")
	assert.Contains(t, output, "Starting services in profile 'healthy': [http-api tcp-api]")
	assert.NotContains(t, output, "service=unreachable")
	assert.NotContains(t, output, "service=crashing")
	assert.NotContains(t, output, "service=slow")

	//nolint:bodyclose // closed by apiJSON
	statusResp := apiRequest(t, http.MethodGet, "/api/v1/status", apiToken)
	status := apiJSON(t, statusResp)

	assert.Equal(t, "healthy", status["profile"])

	processes := serviceProcesses(t)

	assert.Len(t, processes, 2)
	assert.Contains(t, processes, "tcp-api")
	assert.Contains(t, processes, "http-api")
}

func Test_Profile_UnknownFails(t *testing.T) {
	result := RunOnce(t, "testdata/readiness", "run", "no-such-profile", "--no-ui")

	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stderr, "profile not found: no-such-profile")
	assert.NotContains(t, result.Stdout, "Started service")
}
