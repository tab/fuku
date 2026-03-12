package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Lifecycle_RepeatedStartStop(t *testing.T) {
	for i := range 5 {
		t.Run(fmt.Sprintf("cycle-%d", i+1), func(t *testing.T) {
			runner := NewRunner(t, "testdata/default-tier")
			defer runner.Stop()

			err := runner.Start("default")
			require.NoError(t, err)

			err = runner.WaitForRunning(15 * time.Second)
			require.NoError(t, err)

			output := runner.Output()

			assert.Contains(t, output, "profile_resolved profile=default")
			assert.Contains(t, output, "Starting services in profile 'default': [auth-api user-api]")
			assert.NotContains(t, output, "No services found")

			err = runner.Stop()
			require.NoError(t, err)
		})
	}
}

func Test_Lifecycle_RapidRestart(t *testing.T) {
	for i := range 3 {
		t.Run(fmt.Sprintf("cycle-%d", i+1), func(t *testing.T) {
			runner := NewRunner(t, "testdata/default-tier")
			defer runner.Stop()

			err := runner.Start("default")
			require.NoError(t, err)

			err = runner.WaitForRunning(15 * time.Second)
			require.NoError(t, err)

			runner.Stop()

			output := runner.Output()

			assert.Contains(t, output, "service_ready service=auth-api")
			assert.Contains(t, output, "service_ready service=user-api")
			assert.NotContains(t, output, "No services found")
		})
	}
}

func Test_Lifecycle_PreflightCleansUpOrphans(t *testing.T) {
	first := NewRunner(t, "testdata/default-tier")

	err := first.Start("default")
	require.NoError(t, err)

	err = first.WaitForRunning(15 * time.Second)
	require.NoError(t, err)

	// Kill abruptly without graceful shutdown — leaves orphaned processes
	first.cmd.Process.Kill()
	first.cmd.Wait()

	// Second run should clean up orphans via preflight and start normally
	second := NewRunner(t, "testdata/default-tier")
	defer second.Stop()

	err = second.Start("default")
	require.NoError(t, err)

	err = second.WaitForRunning(20 * time.Second)
	require.NoError(t, err)

	output := second.Output()

	assert.Contains(t, output, "profile_resolved profile=default")
	assert.Contains(t, output, "service_ready service=auth-api")
	assert.Contains(t, output, "service_ready service=user-api")
	assert.NotContains(t, output, "No services found")
}
