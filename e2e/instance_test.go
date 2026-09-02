package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_API_Live_IdentifiesTheInstance(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	resp := apiRequest(t, http.MethodGet, "/api/v1/live", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := apiJSON(t, resp)
	assert.Equal(t, "alive", body["status"])
	assert.Equal(t, "fuku", body["product"])
	assert.NotEmpty(t, body["instance"])

	project, ok := body["project"].(string)
	require.True(t, ok)
	assert.Len(t, project, 16)
	assert.NotContains(t, project, "/", "the unauthenticated probe must not disclose the project path")
}

func Test_API_Status_ReportsTheProjectDirectory(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	//nolint:bodyclose // closed by apiJSON
	live := apiJSON(t, apiRequest(t, http.MethodGet, "/api/v1/live", ""))

	//nolint:bodyclose // closed by apiJSON
	status := apiJSON(t, apiRequest(t, http.MethodGet, "/api/v1/status", apiToken))

	assert.Equal(t, live["instance"], status["instance"])

	project, ok := status["project"].(string)
	require.True(t, ok)
	assert.Contains(t, project, "testdata/api")
}

func Test_Run_RefusesASecondInstanceForTheSameProject(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	before := servicePIDs(t)

	result := RunOnce(t, "testdata/api", "run", "default", "--no-ui")

	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stderr, "already running for this project")

	assert.Equal(t, before, servicePIDs(t), "a refused run must not preflight-kill the running services")
}

func Test_Logs_FlagsASocketServingAnotherProject(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	owner := RunOnce(t, "testdata/api", "logs", "--profile", "default", "--tail", "1", "--no-follow")

	assert.Equal(t, 0, owner.ExitCode)
	assert.NotContains(t, owner.Stdout, "serves a different project")

	// yml-config uses the same profile name, so it reaches the socket testdata/api holds
	stranger := RunOnce(t, "testdata/yml-config", "logs", "--profile", "default", "--tail", "1", "--no-follow")

	assert.Equal(t, 0, stranger.ExitCode, "a mismatch is a warning, not a failure")
	assert.Contains(t, stranger.Stdout, "serves a different project")
}
