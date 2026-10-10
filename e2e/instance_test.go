package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serviceProcesses maps each service name to its reported PID and status
func serviceProcesses(t *testing.T) map[string]any {
	t.Helper()

	//nolint:bodyclose // closed by apiJSON
	resp := apiRequest(t, http.MethodGet, "/api/v1/services", apiToken)
	body := apiJSON(t, resp)

	services, ok := body["services"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, services)

	processes := make(map[string]any, len(services))

	for _, s := range services {
		svc := s.(map[string]any)
		processes[svc["name"].(string)] = [2]any{svc["pid"], svc["status"]}
	}

	return processes
}

func Test_Instance_RefusesSecondRun(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	before := serviceProcesses(t)

	result := RunOnce(t, "testdata/api", "run", "default", "--no-ui")

	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stderr, SocketPath(t, "testdata/api"))
	assert.Contains(t, result.Stderr, "fuku logs")
	assert.Equal(t, 1, strings.Count(result.Stderr, "fuku is already running for this project"),
		"the refusal must be printed once, not repeated by the generic startup error")

	assert.Equal(t, before, serviceProcesses(t), "the refused run must leave the first instance's services untouched")

	//nolint:bodyclose // closed by apiJSON
	resp := apiRequest(t, http.MethodGet, "/api/v1/status", apiToken)
	status := apiJSON(t, resp)

	assert.Equal(t, "running", status["phase"])
}

func Test_Instance_RefusesSecondRunWithoutAPI(t *testing.T) {
	t.Setenv("FUKU_API_DISABLED", "1")

	runner := NewRunner(t, "testdata/default-tier")
	defer runner.Stop()

	require.NoError(t, runner.Start("default"))
	require.NoError(t, runner.WaitForRunning(15*time.Second))

	result := RunOnce(t, "testdata/default-tier", "run", "default", "--no-ui")

	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stderr, SocketPath(t, "testdata/default-tier"))
	assert.Equal(t, 1, strings.Count(result.Stderr, "fuku is already running for this project"))

	logs := RunOnce(t, "testdata/default-tier", "logs", "--no-ui", "--no-follow")

	assert.Equal(t, 0, logs.ExitCode, "the first instance must still serve its socket")
	assert.NotContains(t, runner.Output(), "service_stopped")
}
