package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/app/results"
)

func TestNewDisplay(t *testing.T) {
	display := NewDisplay()
	assert.NotNil(t, display)
	assert.NotNil(t, display.services)
	assert.NotNil(t, display.progress)
	assert.NotNil(t, display.analytics)
	assert.True(t, display.isActive)
}

func TestAddService(t *testing.T) {
	display := NewDisplay()

	display.AddService("test-service")

	assert.Equal(t, 1, display.progress.TotalServices)
	assert.Contains(t, display.services, "test-service")

	service := display.services["test-service"]
	assert.Equal(t, "test-service", service.Name)
	assert.Equal(t, results.StatusPending, service.Status)
}

func TestUpdateServiceStatus(t *testing.T) {
	display := NewDisplay()
	display.AddService("test-service")

	display.UpdateServiceStatus("test-service", results.StatusRunning)

	service := display.services["test-service"]
	assert.Equal(t, results.StatusRunning, service.Status)
	assert.Equal(t, 1, display.progress.ServicesStarted)
}


func TestUpdateProgress(t *testing.T) {
	display := NewDisplay()

	display.UpdateProgress(1, 3, "Starting layer 1")

	assert.Equal(t, 1, display.progress.CurrentLayer)
	assert.Equal(t, 3, display.progress.TotalLayers)
	assert.Equal(t, "Starting layer 1", display.progress.CurrentStep)
}

func TestSetServiceError(t *testing.T) {
	display := NewDisplay()
	display.AddService("test-service")

	testErr := assert.AnError
	display.SetServiceError("test-service", testErr)

	service := display.services["test-service"]
	assert.Equal(t, testErr, service.Error)
}

func TestAnalyticsTracking(t *testing.T) {
	display := NewDisplay()
	display.AddService("fast-service")
	display.AddService("slow-service")

	// Simulate fast service
	time.Sleep(1 * time.Millisecond)
	display.UpdateServiceStatus("fast-service", results.StatusRunning)

	// Simulate slow service
	time.Sleep(5 * time.Millisecond)
	display.UpdateServiceStatus("slow-service", results.StatusRunning)

	assert.Equal(t, "fast-service", display.analytics.FastestService)
	assert.Equal(t, "slow-service", display.analytics.SlowestService)
	assert.True(t, display.analytics.FastestTime < display.analytics.SlowestTime)
}

func TestDisplayModeTransition(t *testing.T) {
	display := NewDisplay()

	// Should start in bootstrap mode
	assert.True(t, display.IsBootstrapMode())

	// Switch to logs mode
	display.SwitchToLogsMode()
	assert.False(t, display.IsBootstrapMode())
}

func TestAreAllServicesRunning(t *testing.T) {
	display := NewDisplay()

	// No services initially
	assert.False(t, display.AreAllServicesRunning())

	// Add services
	display.AddService("service1")
	display.AddService("service2")

	// Not all running yet
	assert.False(t, display.AreAllServicesRunning())

	// Set one to running
	display.UpdateServiceStatus("service1", results.StatusRunning)
	assert.False(t, display.AreAllServicesRunning())

	// Set both to running
	display.UpdateServiceStatus("service2", results.StatusRunning)
	assert.True(t, display.AreAllServicesRunning())

	// Set one to failed
	display.UpdateServiceStatus("service1", results.StatusFailed)
	assert.False(t, display.AreAllServicesRunning())
}

func TestGetStatusEmoji(t *testing.T) {
	tests := []struct {
		status   results.ServiceStatus
		emoji    string
	}{
		{results.StatusPending, "⏳"},
		{results.StatusStarting, "🚀"},
		{results.StatusRunning, "✅"},
		{results.StatusFailed, "❌"},
		{results.StatusStopped, "⏹️"},
	}

	for _, tt := range tests {
		t.Run(tt.emoji, func(t *testing.T) {
			assert.Equal(t, tt.emoji, GetStatusEmoji(tt.status))
		})
	}
}

func TestGetSuggestionForError(t *testing.T) {
	tests := []struct {
		error      string
		suggestion string
	}{
		{
			error:      "port 8080 already in use",
			suggestion: "Check if another instance is running with 'lsof -ti:PORT'",
		},
		{
			error:      "permission denied",
			suggestion: "Try running with appropriate permissions",
		},
		{
			error:      "no such file or directory",
			suggestion: "Verify the service directory and Makefile exist",
		},
		{
			error:      "connection refused",
			suggestion: "Check if dependent services are running",
		},
		{
			error:      "operation timed out",
			suggestion: "Service may need more time to start, check logs",
		},
		{
			error:      "unknown error",
			suggestion: "Check service logs for more details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.error, func(t *testing.T) {
			result := getSuggestionForError(tt.error)
			assert.Equal(t, tt.suggestion, result)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a very long string", 10, "this is..."},
		{"exactly10char", 9, "exactl..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}
