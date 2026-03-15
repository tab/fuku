package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Override_MergesServices(t *testing.T) {
	runner := NewRunner(t, "testdata/override")
	defer runner.Stop()

	err := runner.Start("default")
	require.NoError(t, err)

	err = runner.WaitForServiceStarted("auth-api", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForServiceStarted("debug-tool", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	output := runner.Output()

	assert.Contains(t, output, "profile_resolved profile=default")
	assert.Contains(t, output, "service_ready service=auth-api")
	assert.Contains(t, output, "service_ready service=debug-tool")
}

func Test_Override_ConfigFlagSkipsOverride(t *testing.T) {
	runner := NewRunner(t, "testdata/override")
	defer runner.Stop()

	err := runner.StartWithConfig("fuku.yaml", "default")
	require.NoError(t, err)

	err = runner.WaitForServiceStarted("auth-api", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	output := runner.Output()

	assert.Contains(t, output, "service_ready service=auth-api")
	assert.NotContains(t, output, "debug-tool")
}
