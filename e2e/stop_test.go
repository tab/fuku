package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Stop_NoActiveSession(t *testing.T) {
	r := NewStopRunner(t, "testdata/default-tier")

	exitCode, err := r.Run()
	require.NoError(t, err)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, r.Output(), "No active session found")
}
