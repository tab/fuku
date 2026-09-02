package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// servicePIDs reads the current pid of every service in the active profile
func servicePIDs(t *testing.T) map[string]any {
	t.Helper()

	//nolint:bodyclose // closed by apiJSON
	resp := apiRequest(t, http.MethodGet, "/api/v1/services", apiToken)
	body := apiJSON(t, resp)

	services, ok := body["services"].([]any)
	require.True(t, ok)

	pids := make(map[string]any, len(services))

	for _, entry := range services {
		svc, ok := entry.(map[string]any)
		require.True(t, ok)

		pids[svc["name"].(string)] = svc["pid"]
	}

	return pids
}

func Test_API_Logs(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	resp := apiRequest(t, http.MethodGet, "/api/v1/logs?tail=3", apiToken)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := apiJSON(t, resp)
	assert.InDelta(t, 3, body["tail"], 0)

	lines, ok := body["lines"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, lines)
	assert.LessOrEqual(t, len(lines), 3)

	for _, entry := range lines {
		line, ok := entry.(map[string]any)
		require.True(t, ok)

		assert.NotEmpty(t, line["service"])
		assert.NotEmpty(t, line["timestamp"])
	}
}

func Test_API_Logs_RejectsInvalidBounds(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	tests := []struct {
		name  string
		query string
	}{
		{name: "tail is not positive", query: "?tail=0"},
		{name: "tail is not a number", query: "?tail=many"},
		{name: "since is not a duration", query: "?since=yesterday"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := apiRequest(t, http.MethodGet, "/api/v1/logs"+tt.query, apiToken)

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			body := apiJSON(t, resp)
			assert.NotEmpty(t, body["error"])
		})
	}
}

func Test_API_ServiceLogs(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	//nolint:bodyclose // closed by apiJSON
	resp := apiRequest(t, http.MethodGet, "/api/v1/services", apiToken)
	body := apiJSON(t, resp)

	services, ok := body["services"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, services)

	first, ok := services[0].(map[string]any)
	require.True(t, ok)

	name := first["name"].(string)
	id := first["id"].(string)

	//nolint:bodyclose // closed by apiJSON
	logsResp := apiRequest(t, http.MethodGet, "/api/v1/services/"+id+"/logs?tail=5", apiToken)
	logs := apiJSON(t, logsResp)

	lines, ok := logs["lines"].([]any)
	require.True(t, ok)

	for _, entry := range lines {
		line, ok := entry.(map[string]any)
		require.True(t, ok)

		assert.Equal(t, name, line["service"])
	}

	missing := apiRequest(t, http.MethodGet, "/api/v1/services/not-a-service/logs", apiToken)
	defer missing.Body.Close()

	assert.Equal(t, http.StatusNotFound, missing.StatusCode)
}

func Test_Logs_NoFollowReadsBufferAndExits(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	before := servicePIDs(t)

	result := RunOnce(t, "testdata/api", "logs", "--profile", "default", "--tail", "5", "--no-follow")

	assert.Equal(t, 0, result.ExitCode)
	assert.NotEmpty(t, result.Stdout)

	assert.Equal(t, before, servicePIDs(t), "reading the log buffer must not disturb running services")
}

func Test_Logs_SinceBoundsTheReplay(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	full := RunOnce(t, "testdata/api", "logs", "--profile", "default", "--no-follow")
	require.Equal(t, 0, full.ExitCode)

	recent := RunOnce(t, "testdata/api", "logs", "--profile", "default", "--since", "1ms", "--no-follow")

	assert.Equal(t, 0, recent.ExitCode)
	assert.Less(t, len(recent.Stdout), len(full.Stdout), "a 1ms window must replay less than the whole buffer")
	assert.NotContains(t, recent.Stdout, "ctrl+c", "a bounded read has already exited")
}
