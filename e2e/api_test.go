package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	apiBase  = "http://127.0.0.1:19876"
	apiToken = "test-token"
)

func apiRequest(t *testing.T, method, path, token string) *http.Response {
	t.Helper()

	url := apiBase + path

	req, err := http.NewRequest(method, url, nil)
	require.NoError(t, err)

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Do(req)
	require.NoError(t, err)

	return resp
}

func apiJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(body, &result))

	return result
}

func startAPIRunner(t *testing.T) *Runner {
	t.Helper()

	runner := NewRunner(t, "testdata/api")

	err := runner.Start("default")
	require.NoError(t, err)

	err = runner.WaitForRunning(30 * time.Second)
	require.NoError(t, err)

	return runner
}

func Test_API_Status(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	resp := apiRequest(t, http.MethodGet, "/api/v1/status", apiToken)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := apiJSON(t, resp)
	assert.Equal(t, "default", body["profile"])
	assert.Equal(t, "running", body["phase"])
	assert.NotEmpty(t, body["version"])

	services, ok := body["services"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 2, services["total"], 0)
	assert.InDelta(t, 2, services["running"], 0)
}

func Test_API_ListServices(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	resp := apiRequest(t, http.MethodGet, "/api/v1/services", apiToken)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := apiJSON(t, resp)
	services, ok := body["services"].([]any)
	require.True(t, ok)
	require.Len(t, services, 2)

	names := make([]string, len(services))
	for i, s := range services {
		svc := s.(map[string]any)
		names[i] = svc["name"].(string)

		assert.NotEmpty(t, svc["id"])
		assert.Equal(t, "running", svc["status"])
		assert.NotZero(t, svc["pid"])
	}

	assert.Contains(t, names, "auth-api")
	assert.Contains(t, names, "user-api")
}

func Test_API_GetService(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	//nolint:bodyclose // closed by apiJSON
	listResp := apiRequest(t, http.MethodGet, "/api/v1/services", apiToken)
	listBody := apiJSON(t, listResp)
	services := listBody["services"].([]any)
	firstService := services[0].(map[string]any)
	serviceID := firstService["id"].(string)

	resp := apiRequest(t, http.MethodGet, "/api/v1/services/"+serviceID, apiToken)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := apiJSON(t, resp)
	assert.Equal(t, serviceID, body["id"])
	assert.Equal(t, "running", body["status"])
	assert.NotEmpty(t, body["name"])
	assert.NotZero(t, body["pid"])
}

func Test_API_GetService_NotFound(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	resp := apiRequest(t, http.MethodGet, "/api/v1/services/nonexistent-uuid", apiToken)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	body := apiJSON(t, resp)
	assert.Equal(t, "service not found", body["error"])
}

func Test_API_StopAndStartService(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	//nolint:bodyclose // closed by apiJSON
	listResp := apiRequest(t, http.MethodGet, "/api/v1/services", apiToken)
	listBody := apiJSON(t, listResp)
	services := listBody["services"].([]any)
	svc := services[0].(map[string]any)
	serviceID := svc["id"].(string)
	serviceName := svc["name"].(string)

	stopResp := apiRequest(t, http.MethodPost, "/api/v1/services/"+serviceID+"/stop", apiToken)

	assert.Equal(t, http.StatusAccepted, stopResp.StatusCode)

	stopBody := apiJSON(t, stopResp)
	assert.Equal(t, serviceID, stopBody["id"])
	assert.Equal(t, serviceName, stopBody["name"])
	assert.Equal(t, "stop", stopBody["action"])

	require.Eventually(t, func() bool {
		//nolint:bodyclose // closed by apiJSON
		resp := apiRequest(t, http.MethodGet, "/api/v1/services/"+serviceID, apiToken)
		body := apiJSON(t, resp)

		return body["status"] == "stopped"
	}, 10*time.Second, 200*time.Millisecond)

	startResp := apiRequest(t, http.MethodPost, "/api/v1/services/"+serviceID+"/start", apiToken)

	assert.Equal(t, http.StatusAccepted, startResp.StatusCode)

	startBody := apiJSON(t, startResp)
	assert.Equal(t, serviceID, startBody["id"])
	assert.Equal(t, "start", startBody["action"])

	require.Eventually(t, func() bool {
		//nolint:bodyclose // closed by apiJSON
		resp := apiRequest(t, http.MethodGet, "/api/v1/services/"+serviceID, apiToken)
		body := apiJSON(t, resp)

		return body["status"] == "running"
	}, 10*time.Second, 200*time.Millisecond)
}

func Test_API_RestartService(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	//nolint:bodyclose // closed by apiJSON
	listResp := apiRequest(t, http.MethodGet, "/api/v1/services", apiToken)
	listBody := apiJSON(t, listResp)
	services := listBody["services"].([]any)
	svc := services[0].(map[string]any)
	serviceID := svc["id"].(string)
	originalPID := svc["pid"]

	restartResp := apiRequest(t, http.MethodPost, "/api/v1/services/"+serviceID+"/restart", apiToken)

	assert.Equal(t, http.StatusAccepted, restartResp.StatusCode)

	restartBody := apiJSON(t, restartResp)
	assert.Equal(t, "restart", restartBody["action"])

	require.Eventually(t, func() bool {
		//nolint:bodyclose // closed by apiJSON
		resp := apiRequest(t, http.MethodGet, "/api/v1/services/"+serviceID, apiToken)
		body := apiJSON(t, resp)

		return body["status"] == "running" && body["pid"] != originalPID
	}, 15*time.Second, 200*time.Millisecond)
}

func Test_API_StopConflict(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	//nolint:bodyclose // closed by apiJSON
	listResp := apiRequest(t, http.MethodGet, "/api/v1/services", apiToken)
	listBody := apiJSON(t, listResp)
	services := listBody["services"].([]any)
	serviceID := services[0].(map[string]any)["id"].(string)

	stopResp := apiRequest(t, http.MethodPost, "/api/v1/services/"+serviceID+"/stop", apiToken)
	stopResp.Body.Close()

	require.Eventually(t, func() bool {
		//nolint:bodyclose // closed by apiJSON
		resp := apiRequest(t, http.MethodGet, "/api/v1/services/"+serviceID, apiToken)
		body := apiJSON(t, resp)

		return body["status"] == "stopped"
	}, 10*time.Second, 200*time.Millisecond)

	resp := apiRequest(t, http.MethodPost, "/api/v1/services/"+serviceID+"/stop", apiToken)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	body := apiJSON(t, resp)
	assert.Equal(t, "service is not running", body["error"])
}

func Test_API_StartConflict(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	//nolint:bodyclose // closed by apiJSON
	listResp := apiRequest(t, http.MethodGet, "/api/v1/services", apiToken)
	listBody := apiJSON(t, listResp)
	services := listBody["services"].([]any)
	serviceID := services[0].(map[string]any)["id"].(string)

	resp := apiRequest(t, http.MethodPost, "/api/v1/services/"+serviceID+"/start", apiToken)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	body := apiJSON(t, resp)
	assert.Equal(t, "service cannot be started", body["error"])
}

func Test_API_Unauthorized(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "no token",
			token: "",
		},
		{
			name:  "wrong token",
			token: "wrong-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := apiRequest(t, http.MethodGet, "/api/v1/status", tt.token)

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

			body := apiJSON(t, resp)
			assert.Equal(t, "unauthorized", body["error"])
		})
	}
}

func Test_API_StatusCounts(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	//nolint:bodyclose // closed by apiJSON
	listResp := apiRequest(t, http.MethodGet, "/api/v1/services", apiToken)
	listBody := apiJSON(t, listResp)
	services := listBody["services"].([]any)
	serviceID := services[0].(map[string]any)["id"].(string)

	stopResp := apiRequest(t, http.MethodPost, "/api/v1/services/"+serviceID+"/stop", apiToken)
	stopResp.Body.Close()

	require.Eventually(t, func() bool {
		//nolint:bodyclose // closed by apiJSON
		resp := apiRequest(t, http.MethodGet, "/api/v1/services/"+serviceID, apiToken)
		body := apiJSON(t, resp)

		return body["status"] == "stopped"
	}, 10*time.Second, 200*time.Millisecond)

	//nolint:bodyclose // closed by apiJSON
	resp := apiRequest(t, http.MethodGet, "/api/v1/status", apiToken)
	body := apiJSON(t, resp)
	counts := body["services"].(map[string]any)

	assert.InDelta(t, 2, counts["total"], 0)
	assert.InDelta(t, 1, counts["running"], 0)
	assert.InDelta(t, 1, counts["stopped"], 0)
}

func Test_API_LiveProbe(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	resp := apiRequest(t, http.MethodGet, "/api/v1/live", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := apiJSON(t, resp)
	assert.Equal(t, "alive", body["status"])
}

func Test_API_ReadyProbe(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	resp := apiRequest(t, http.MethodGet, "/api/v1/ready", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := apiJSON(t, resp)
	assert.Equal(t, "ready", body["status"])
}

func Test_API_RestartStoppedService(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	//nolint:bodyclose // closed by apiJSON
	listResp := apiRequest(t, http.MethodGet, "/api/v1/services", apiToken)
	listBody := apiJSON(t, listResp)
	services := listBody["services"].([]any)
	serviceID := services[0].(map[string]any)["id"].(string)

	stopResp := apiRequest(t, http.MethodPost, "/api/v1/services/"+serviceID+"/stop", apiToken)
	stopResp.Body.Close()

	require.Eventually(t, func() bool {
		//nolint:bodyclose // closed by apiJSON
		resp := apiRequest(t, http.MethodGet, "/api/v1/services/"+serviceID, apiToken)
		body := apiJSON(t, resp)

		return body["status"] == "stopped"
	}, 10*time.Second, 200*time.Millisecond)

	restartResp := apiRequest(t, http.MethodPost, "/api/v1/services/"+serviceID+"/restart", apiToken)

	assert.Equal(t, http.StatusAccepted, restartResp.StatusCode)

	restartBody := apiJSON(t, restartResp)
	assert.Equal(t, "restart", restartBody["action"])

	require.Eventually(t, func() bool {
		//nolint:bodyclose // closed by apiJSON
		resp := apiRequest(t, http.MethodGet, "/api/v1/services/"+serviceID, apiToken)
		body := apiJSON(t, resp)

		return body["status"] == "running"
	}, 15*time.Second, 200*time.Millisecond)
}

func Test_API_ServiceHasUUID(t *testing.T) {
	runner := startAPIRunner(t)
	defer runner.Stop()

	//nolint:bodyclose // closed by apiJSON
	resp := apiRequest(t, http.MethodGet, "/api/v1/services", apiToken)
	body := apiJSON(t, resp)
	services := body["services"].([]any)

	for _, s := range services {
		svc := s.(map[string]any)
		id := svc["id"].(string)

		assert.Len(t, id, 36, "service %s should have a UUID", svc["name"])
		assert.Contains(t, id, "-")
	}
}
