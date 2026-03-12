package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_CustomCommand_BothServicesStart(t *testing.T) {
	runner := NewRunner(t, "testdata/custom-command")
	defer runner.Stop()

	err := runner.Start("default")
	require.NoError(t, err)

	err = runner.WaitForServiceStarted("with-makefile", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForServiceStarted("with-command", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	output := runner.Output()

	assert.Contains(t, output, "service_ready service=with-makefile")
	assert.Contains(t, output, "service_ready service=with-command")
}

func Test_CustomCommand_GracefulShutdown(t *testing.T) {
	runner := NewRunner(t, "testdata/custom-command")
	defer runner.Stop()

	err := runner.Start("default")
	require.NoError(t, err)

	err = runner.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	err = runner.Stop()
	require.NoError(t, err)

	output := runner.Output()

	assert.Contains(t, output, "signal signal=terminated")
	assert.Contains(t, output, "Received signal terminated, shutting down services")
}
