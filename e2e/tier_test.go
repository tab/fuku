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
	foundationStartIdx := indexOf(output, "tier_starting {tier: foundation")
	foundationReadyIdx := indexOf(output, "tier_ready {name: foundation}")
	platformStartIdx := indexOf(output, "tier_starting {tier: platform")
	platformReadyIdx := indexOf(output, "tier_ready {name: platform}")
	edgeStartIdx := indexOf(output, "tier_starting {tier: edge")
	edgeReadyIdx := indexOf(output, "tier_ready {name: edge}")

	// All tiers should be present
	assert.Greater(t, foundationStartIdx, -1, "foundation tier_starting should be present")
	assert.Greater(t, foundationReadyIdx, -1, "foundation tier_ready should be present")
	assert.Greater(t, platformStartIdx, -1, "platform tier_starting should be present")
	assert.Greater(t, platformReadyIdx, -1, "platform tier_ready should be present")
	assert.Greater(t, edgeStartIdx, -1, "edge tier_starting should be present")
	assert.Greater(t, edgeReadyIdx, -1, "edge tier_ready should be present")

	// Tiers should start in order
	assert.Less(t, foundationStartIdx, foundationReadyIdx, "foundation should start before ready")
	assert.Less(t, foundationReadyIdx, platformStartIdx, "foundation should complete before platform starts")
	assert.Less(t, platformStartIdx, platformReadyIdx, "platform should start before ready")
	assert.Less(t, platformReadyIdx, edgeStartIdx, "platform should complete before edge starts")
	assert.Less(t, edgeStartIdx, edgeReadyIdx, "edge should start before ready")

	// Service events
	assert.Contains(t, output, "service_ready {service: postgres")
	assert.Contains(t, output, "service_ready {service: redis")
	assert.Contains(t, output, "service_ready {service: gateway")
}
