package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Watch_RestartOnFileChange(t *testing.T) {
	runner := NewRunner(t, "testdata/watch-service")
	defer runner.Stop()

	err := runner.Start("default")
	require.NoError(t, err)

	err = runner.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	err = runner.WaitForLog("Started watching service 'worker'", 10*time.Second)
	require.NoError(t, err)

	err = runner.TouchFile("worker/main.go")
	require.NoError(t, err)

	err = runner.WaitForLog("watch_triggered {service: worker", 5*time.Second)
	require.NoError(t, err)

	err = runner.WaitForLog("File change detected for service 'worker'", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForLog("service_restarting {service: worker", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForLog("service_ready {service: worker", 20*time.Second)
	require.NoError(t, err)

	output := runner.Output()

	restartIdx := indexOf(output, "service_restarting {service: worker")
	require.Greater(t, restartIdx, -1, "service_restarting event should be present")

	// Find service_ready after the restart event
	afterRestart := output[restartIdx:]
	readyAfterRestart := strings.Index(afterRestart, "service_ready {service: worker")

	assert.Greater(t, readyAfterRestart, -1, "service_ready should appear after service_restarting")

	watchIdx := indexOf(output, "watch_triggered {service: worker")
	fileChangeIdx := indexOf(output, "File change detected for service 'worker'")

	assert.Greater(t, watchIdx, -1, "watch_triggered event should be present")
	assert.Greater(t, fileChangeIdx, -1, "file change log should be present")

	assert.Less(t, watchIdx, fileChangeIdx, "watch_triggered should appear before file change log")
	assert.Less(t, fileChangeIdx, restartIdx, "file change log should appear before service_restarting")
}
