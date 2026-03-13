package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_YmlConfig_StartServices(t *testing.T) {
	runner := NewRunner(t, "testdata/yml-config")
	defer runner.Stop()

	err := runner.Start("default")
	require.NoError(t, err)

	err = runner.WaitForServiceStarted("echo-api", 10*time.Second)
	require.NoError(t, err)

	err = runner.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	output := runner.Output()

	assert.Contains(t, output, "profile_resolved profile=default")
	assert.Contains(t, output, "service_ready service=echo-api")
}
