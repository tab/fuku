package e2e

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logEntry is the part of a JSON log line the tests read
type logEntry struct {
	Level     string `json:"level"`
	Component string `json:"component"`
	Service   string `json:"service"`
	Stream    string `json:"stream"`
	Message   string `json:"message"`
}

// logEntries decodes every stdout line as a JSON log entry that names its level, component and message
func logEntries(t *testing.T, output string) []logEntry {
	t.Helper()

	var entries []logEntry

	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		var entry logEntry

		require.NoError(t, json.Unmarshal([]byte(line), &entry), "every stdout line must be a JSON object: %q", line)
		assert.NotEmpty(t, entry.Level, line)
		assert.NotEmpty(t, entry.Component, line)
		assert.NotEmpty(t, entry.Message, line)

		entries = append(entries, entry)
	}

	return entries
}

func Test_Logging_JSONFormat(t *testing.T) {
	runner := NewRunner(t, "testdata/logging")
	defer runner.Stop()

	require.NoError(t, runner.Start("default"))
	require.NoError(t, runner.WaitForRunning(15*time.Second))
	require.NoError(t, runner.WaitForLog(`"message":"Service ready"`, 10*time.Second))
	require.NoError(t, runner.Stop())

	entries := logEntries(t, runner.Output())

	assert.Contains(t, entries, logEntry{
		Level:     "info",
		Component: "PROCESS",
		Service:   "echo-api",
		Stream:    "STDOUT",
		Message:   "Service ready",
	})
}

func Test_Logging_OutputStreamFilter(t *testing.T) {
	runner := NewRunner(t, "testdata/logging")
	defer runner.Stop()

	require.NoError(t, runner.Start("default"))
	require.NoError(t, runner.WaitForRunning(15*time.Second))
	require.NoError(t, runner.WaitForLog("shown stderr", 10*time.Second))
	require.NoError(t, runner.WaitForLog(`"message":"Service ready"`, 10*time.Second))
	require.NoError(t, runner.Stop())

	output := runner.Output()
	entries := logEntries(t, output)

	assert.Contains(t, entries, logEntry{
		Level:     "info",
		Component: "PROCESS",
		Service:   "quiet-api",
		Stream:    "STDERR",
		Message:   "shown stderr",
	})
	assert.NotContains(t, output, "hidden stdout")
}
