package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Tier_StartsInOrder(t *testing.T) {
	runner := NewRunner(t, "testdata/tier")
	defer runner.Stop()

	err := runner.Start("default")
	require.NoError(t, err)

	err = runner.WaitForRunning(30 * time.Second)
	require.NoError(t, err)

	output := runner.Output()

	// Tier sequence: foundation -> platform -> edge
	foundationStartIdx := indexOf(output, "tier_starting tier=foundation")
	platformStartIdx := indexOf(output, "tier_starting tier=platform")
	edgeStartIdx := indexOf(output, "tier_starting tier=edge")

	assert.Greater(t, foundationStartIdx, -1, "foundation tier_starting should be present")
	assert.Greater(t, platformStartIdx, -1, "platform tier_starting should be present")
	assert.Greater(t, edgeStartIdx, -1, "edge tier_starting should be present")

	// Tiers should start in order
	assert.Less(t, foundationStartIdx, platformStartIdx, "foundation should start before platform")
	assert.Less(t, platformStartIdx, edgeStartIdx, "platform should start before edge")

	// Each tier must be ready before the next tier starts
	foundationSection := output[foundationStartIdx:platformStartIdx]
	platformSection := output[platformStartIdx:edgeStartIdx]
	edgeSection := output[edgeStartIdx:]

	assert.Contains(t, foundationSection, "tier_ready", "foundation should be ready before platform starts")
	assert.Contains(t, platformSection, "tier_ready", "platform should be ready before edge starts")
	assert.Contains(t, edgeSection, "tier_ready", "edge should become ready")

	// Service events
	assert.Contains(t, output, "service_ready")
	assert.Contains(t, output, "service=postgres")
	assert.Contains(t, output, "service=redis")
	assert.Contains(t, output, "service=gateway")
}
