package results

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ServiceResult_UpdateStatus(t *testing.T) {
	result := NewServiceResult("test-service")

	assert.Equal(t, StatusPending, result.Status)

	result.UpdateStatus(StatusRunning)
	assert.Equal(t, StatusRunning, result.Status)
}

func Test_ResultsTracker_AddService(t *testing.T) {
	tracker := NewResultsTracker()

	result := tracker.AddService("test-service")
	assert.NotNil(t, result)
	assert.Equal(t, "test-service", result.Name)
	assert.Equal(t, StatusPending, result.Status)
}
