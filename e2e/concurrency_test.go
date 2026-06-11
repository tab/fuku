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
	concurrencyAPIBase  = "http://127.0.0.1:19877"
	concurrencyAPIToken = "test-token"
)

// statusSample holds one observation of the status endpoint during startup
type statusSample struct {
	phase  string
	counts map[string]int
}

// fetchStatusSample polls the status endpoint, returning false while the API is not reachable yet
func fetchStatusSample() (statusSample, bool) {
	req, err := http.NewRequest(http.MethodGet, concurrencyAPIBase+"/api/v1/status", nil)
	if err != nil {
		return statusSample{}, false
	}

	req.Header.Set("Authorization", "Bearer "+concurrencyAPIToken)

	client := &http.Client{Timeout: time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return statusSample{}, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return statusSample{}, false
	}

	var parsed struct {
		Phase    string         `json:"phase"`
		Services map[string]int `json:"services"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil {
		return statusSample{}, false
	}

	return statusSample{phase: parsed.Phase, counts: parsed.Services}, true
}

func Test_Concurrency_WorkersSerializeStartup(t *testing.T) {
	runner := NewRunner(t, "testdata/concurrency")
	defer runner.Stop()

	err := runner.Start("default")
	require.NoError(t, err)

	// Sample the status endpoint during startup: with workers: 1 the queue
	// must drain one service at a time while the rest report pending
	var (
		maxStarting int
		sawQueue    bool
	)

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)

sampling:
	for {
		select {
		case <-timeout:
			break sampling
		case <-ticker.C:
			sample, ok := fetchStatusSample()
			if !ok {
				continue
			}

			maxStarting = max(maxStarting, sample.counts["starting"])

			if sample.counts["starting"] == 1 && sample.counts["pending"] >= 1 {
				sawQueue = true
			}

			if sample.phase == "running" {
				break sampling
			}
		}
	}

	err = runner.WaitForRunning(30 * time.Second)
	require.NoError(t, err)

	assert.LessOrEqual(t, maxStarting, 1, "workers: 1 must never start more than one service at a time")
	require.True(t, sawQueue, "expected to observe one starting service while others were pending\nOutput:\n%s", runner.Output())

	sample, ok := fetchStatusSample()
	require.True(t, ok)
	assert.Equal(t, "running", sample.phase)
	assert.Equal(t, 3, sample.counts["total"])
	assert.Equal(t, 3, sample.counts["running"])
	assert.Equal(t, 0, sample.counts["pending"])
	assert.Equal(t, 0, sample.counts["starting"])
}
