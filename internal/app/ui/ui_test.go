package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/tracker"
)

func Test_NewDisplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	displayInstance := NewDisplay(mockTracker)
	assert.NotNil(t, displayInstance)

	instance, ok := displayInstance.(*display)
	assert.True(t, ok)
	assert.NotNil(t, instance)
	assert.True(t, displayInstance.IsBootstrap())
}

func Test_Add(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	mockResult := tracker.NewMockResult(ctrl)

	mockTracker.EXPECT().Add("test-service").Return(mockResult)
	mockResult.EXPECT().GetStatus().AnyTimes().Return(tracker.StatusPending)

	d := NewDisplay(mockTracker)
	d.Add("test-service")

	assert.Equal(t, 1, d.GetTotalServices())
}

func Test_Update(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	mockResult := tracker.NewMockResult(ctrl)

	mockTracker.EXPECT().Add("test-service").Return(mockResult)
	mockResult.EXPECT().GetStatus().Return(tracker.StatusPending)
	mockResult.EXPECT().SetStatus(tracker.StatusRunning)

	d := NewDisplay(mockTracker)
	d.Add("test-service")
	d.Update("test-service", tracker.StatusRunning)

	assert.Equal(t, 1, d.GetServicesStarted())
}

func Test_SetProgress(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	d := NewDisplay(mockTracker)

	d.SetProgress(1, 3, "Starting layer 1")

	assert.Equal(t, 1, d.GetCurrentLayer())
	assert.Equal(t, 3, d.GetTotalLayers())
	assert.Equal(t, "Starting layer 1", d.GetCurrentStep())
}

func Test_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	mockResult := tracker.NewMockResult(ctrl)

	mockTracker.EXPECT().Add("test-service").Return(mockResult)
	testErr := assert.AnError
	mockResult.EXPECT().SetError(testErr)

	d := NewDisplay(mockTracker)
	d.Add("test-service")
	d.Error("test-service", testErr)
}

func Test_AnalyticsTracking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	mockResult1 := tracker.NewMockResult(ctrl)
	mockResult2 := tracker.NewMockResult(ctrl)

	mockTracker.EXPECT().Add("fast-service").Return(mockResult1)
	mockTracker.EXPECT().Add("slow-service").Return(mockResult2)
	mockResult1.EXPECT().GetStatus().Return(tracker.StatusPending)
	mockResult1.EXPECT().SetStatus(tracker.StatusRunning)
	mockResult2.EXPECT().GetStatus().Return(tracker.StatusPending)
	mockResult2.EXPECT().SetStatus(tracker.StatusRunning)

	d := NewDisplay(mockTracker)
	d.Add("fast-service")
	d.Add("slow-service")

	time.Sleep(1 * time.Millisecond)
	d.Update("fast-service", tracker.StatusRunning)

	time.Sleep(5 * time.Millisecond)
	d.Update("slow-service", tracker.StatusRunning)

	analytics := d.GetAnalytics()
	assert.Equal(t, "fast-service", analytics.FastestService)
	assert.Equal(t, "slow-service", analytics.SlowestService)
	assert.True(t, analytics.FastestTime < analytics.SlowestTime)
}

func Test_ModeTransition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	d := NewDisplay(mockTracker)
	assert.True(t, d.IsBootstrap())

	d.SwitchToLogs()
	assert.False(t, d.IsBootstrap())
}

func Test_GetStatusSymbol(t *testing.T) {
	tests := []struct {
		status tracker.Status
		name   string
	}{
		{tracker.StatusPending, "pending"},
		{tracker.StatusStarting, "starting"},
		{tracker.StatusRunning, "running"},
		{tracker.StatusFailed, "failed"},
		{tracker.StatusStopped, "stopped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStatusSymbol(tt.status)
			// Just verify we get a non-empty colored string
			assert.NotEmpty(t, result)
			assert.Contains(t, result, "\033[") // Contains ANSI color codes
		})
	}
}

func Test_GetStatusSymbol_Default(t *testing.T) {
	invalidStatus := tracker.Status(999)
	result := GetStatusSymbol(invalidStatus)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "\033[") // Contains ANSI color codes
}

func Test_BufferLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	d := NewDisplay(mockTracker)

	assert.True(t, d.IsBootstrap())
	d.BufferLog("test log message")

	d.SwitchToLogs()

	d.BufferLog("direct message")
}

func Test_ProgressBar(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	d := NewDisplay(mockTracker)

	tests := []struct {
		name     string
		current  int
		total    int
		width    int
		expected string
	}{
		{
			name:     "Empty progress",
			current:  0,
			total:    10,
			width:    10,
			expected: "░░░░░░░░░░ 0/10 (0%)",
		},
		{
			name:     "Half progress",
			current:  5,
			total:    10,
			width:    10,
			expected: "█████░░░░░ 5/10 (50%)",
		},
		{
			name:     "Full progress",
			current:  10,
			total:    10,
			width:    10,
			expected: "██████████ 10/10 (100%)",
		},
		{
			name:     "Zero total",
			current:  5,
			total:    0,
			width:    5,
			expected: "░░░░░",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.ProgressBar(tt.current, tt.total, tt.width)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_ShowLayer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	mockResult1 := tracker.NewMockResult(ctrl)
	mockResult2 := tracker.NewMockResult(ctrl)

	mockTracker.EXPECT().Add("service1").Return(mockResult1)
	mockTracker.EXPECT().Add("service2").Return(mockResult2)
	mockResult1.EXPECT().GetStatus().Return(tracker.StatusPending)
	mockResult2.EXPECT().GetStatus().Return(tracker.StatusPending)

	d := NewDisplay(mockTracker)
	d.Add("service1")
	d.Add("service2")

	services := []string{"service1", "service2"}
	d.ShowLayer(0, services)
}

func Test_UpdateLayer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	mockResult := tracker.NewMockResult(ctrl)

	mockTracker.EXPECT().Add("service1").Return(mockResult)
	mockResult.EXPECT().GetStatus().Return(tracker.StatusPending)

	d := NewDisplay(mockTracker)
	d.Add("service1")

	services := []string{"service1"}
	d.UpdateLayer(0, services)
}

func Test_ShowSummary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	mockResult1 := tracker.NewMockResult(ctrl)
	mockResult2 := tracker.NewMockResult(ctrl)

	mockTracker.EXPECT().Add("service1").Return(mockResult1)
	mockTracker.EXPECT().Add("service2").Return(mockResult2)
	mockResult1.EXPECT().GetStatus().Return(tracker.StatusPending).AnyTimes()
	mockResult2.EXPECT().GetStatus().Return(tracker.StatusPending).AnyTimes()
	mockResult1.EXPECT().SetStatus(tracker.StatusRunning)
	mockResult2.EXPECT().SetStatus(tracker.StatusFailed)
	mockResult1.EXPECT().GetStatus().Return(tracker.StatusRunning).AnyTimes()
	mockResult2.EXPECT().GetStatus().Return(tracker.StatusFailed).AnyTimes()

	d := NewDisplay(mockTracker)
	d.Add("service1")
	d.Add("service2")
	d.Update("service1", tracker.StatusRunning)
	d.Update("service2", tracker.StatusFailed)

	d.ShowSummary()
}

func Test_ShowSummary_NotBootstrapMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	mockResult := tracker.NewMockResult(ctrl)

	mockTracker.EXPECT().Add("service1").Return(mockResult)
	mockResult.EXPECT().GetStatus().Return(tracker.StatusPending).AnyTimes()
	mockResult.EXPECT().SetStatus(tracker.StatusRunning)

	d := NewDisplay(mockTracker)
	d.Add("service1")
	d.Update("service1", tracker.StatusRunning)

	d.SwitchToLogs()
	d.ShowSummary()
}

func Test_ShowError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	d := NewDisplay(mockTracker)

	d.ShowError("test-service", assert.AnError)
}

func Test_ShowSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	mockResult := tracker.NewMockResult(ctrl)

	mockTracker.EXPECT().Add("service1").Return(mockResult)
	mockResult.EXPECT().GetStatus().Return(tracker.StatusPending).AnyTimes()
	mockResult.EXPECT().SetStatus(tracker.StatusRunning)
	mockResult.EXPECT().GetStatus().Return(tracker.StatusRunning).AnyTimes()

	d := NewDisplay(mockTracker)
	d.Add("service1")
	d.Update("service1", tracker.StatusRunning)

	d.ShowSuccess()

	assert.False(t, d.IsBootstrap())
}

func TestDisplay_ShowSuccess_WithLayerDurations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	mockResult := tracker.NewMockResult(ctrl)

	mockTracker.EXPECT().Add("service1").Return(mockResult)
	mockResult.EXPECT().GetStatus().Return(tracker.StatusPending).AnyTimes()
	mockResult.EXPECT().SetStatus(tracker.StatusRunning)
	mockResult.EXPECT().GetStatus().Return(tracker.StatusRunning).AnyTimes()

	d := NewDisplay(mockTracker)
	d.Add("service1")
	d.Update("service1", tracker.StatusRunning)

	d.SetProgress(1, 3, "Layer 1")
	time.Sleep(1 * time.Millisecond)
	d.SetProgress(2, 3, "Layer 2")
	time.Sleep(1 * time.Millisecond)
	d.SetProgress(3, 3, "Layer 3")

	d.ShowSuccess()

	assert.False(t, d.IsBootstrap())

	analytics := d.GetAnalytics()
	assert.NotEmpty(t, analytics.LayerDurations)
}

func Test_LogBuffer_Add(t *testing.T) {
	buffer := &LogBuffer{}

	buffer.Add("message 1")
	buffer.Add("message 2")

	assert.Equal(t, 2, len(buffer.lines))
	assert.Equal(t, "message 1", buffer.lines[0])
	assert.Equal(t, "message 2", buffer.lines[1])
}

func Test_LogBuffer_Flush(t *testing.T) {
	buffer := &LogBuffer{}
	buffer.Add("message 1")
	buffer.Add("message 2")

	buffer.Flush()

	assert.Equal(t, 0, len(buffer.lines))
}

func Test_TruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a very long string", 10, "this is..."},
		{"exactly10char", 9, "exactl..."},
		{"", 5, ""},
		{"test", 4, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_getSuggestionForError(t *testing.T) {
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
		{
			error:      "PORT already in use with different case",
			suggestion: "Check if another instance is running with 'lsof -ti:PORT'",
		},
		{
			error:      "PERMISSION DENIED with different case",
			suggestion: "Try running with appropriate permissions",
		},
		{
			error:      "service timed out during startup",
			suggestion: "Service may need more time to start, check logs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.error, func(t *testing.T) {
			result := getSuggestionForError(tt.error)
			assert.Equal(t, tt.suggestion, result)
		})
	}
}

func Test_LayerProgressWithLayers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	d := NewDisplay(mockTracker)

	d.SetProgress(1, 3, "Layer 1")
	time.Sleep(1 * time.Millisecond)

	d.SetProgress(2, 3, "Layer 2")

	analytics := d.GetAnalytics()
	assert.Contains(t, analytics.LayerDurations, 0)
	assert.True(t, analytics.LayerDurations[0] > 0)
}

func Test_ServiceStatusTransitions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	mockResult := tracker.NewMockResult(ctrl)

	mockTracker.EXPECT().Add("test-service").Return(mockResult)
	mockResult.EXPECT().GetStatus().Return(tracker.StatusPending).AnyTimes()
	mockResult.EXPECT().SetStatus(gomock.Any()).AnyTimes()

	d := NewDisplay(mockTracker)
	d.Add("test-service")

	statuses := []tracker.Status{
		tracker.StatusStarting,
		tracker.StatusRunning,
		tracker.StatusFailed,
		tracker.StatusStopped,
	}

	for _, status := range statuses {
		d.Update("test-service", status)
	}
}

func Test_MultipleServicesAnalytics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := tracker.NewMockTracker(ctrl)
	mockResult1 := tracker.NewMockResult(ctrl)
	mockResult2 := tracker.NewMockResult(ctrl)
	mockResult3 := tracker.NewMockResult(ctrl)

	mockTracker.EXPECT().Add("service1").Return(mockResult1)
	mockTracker.EXPECT().Add("service2").Return(mockResult2)
	mockTracker.EXPECT().Add("service3").Return(mockResult3)
	mockResult1.EXPECT().GetStatus().Return(tracker.StatusPending).AnyTimes()
	mockResult2.EXPECT().GetStatus().Return(tracker.StatusPending).AnyTimes()
	mockResult3.EXPECT().GetStatus().Return(tracker.StatusPending).AnyTimes()
	mockResult1.EXPECT().SetStatus(tracker.StatusRunning)
	mockResult2.EXPECT().SetStatus(tracker.StatusRunning)
	mockResult3.EXPECT().SetStatus(tracker.StatusRunning)

	d := NewDisplay(mockTracker)

	services := []string{"service1", "service2", "service3"}
	for _, service := range services {
		d.Add(service)
		time.Sleep(1 * time.Millisecond)
		d.Update(service, tracker.StatusRunning)
	}

	analytics := d.GetAnalytics()
	assert.NotEmpty(t, analytics.FastestService)
	assert.NotEmpty(t, analytics.SlowestService)
	assert.True(t, analytics.FastestTime <= analytics.SlowestTime)
}
