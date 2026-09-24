package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_renderError(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "no error",
			text:     "",
			expected: "",
		},
		{
			name:     "port already in use",
			text:     contracts.ErrPortAlreadyInUse.Error(),
			expected: "port already in use",
		},
		{
			name:     "max retries exceeded",
			text:     contracts.ErrMaxRetriesExceeded.Error(),
			expected: "max retries exceeded",
		},
		{
			name:     "process exited",
			text:     contracts.ErrProcessExited.Error(),
			expected: "process exited",
		},
		{
			name:     "readiness timeout",
			text:     contracts.ErrReadinessTimeout.Error(),
			expected: "readiness timeout",
		},
		{
			name:     "failed to start command",
			text:     contracts.ErrFailedToStartCommand.Error(),
			expected: "failed to start",
		},
		{
			name:     "service not found",
			text:     contracts.ErrServiceNotFound.Error(),
			expected: "service not found",
		},
		{
			name:     "service directory not exist",
			text:     contracts.ErrServiceDirectoryNotExist.Error(),
			expected: "directory not found",
		},
		{
			name:     "a chain carried as text names its first sentinel",
			text:     "max retry attempts exceeded after 2 attempts: readiness check failed: process exited before readiness",
			expected: "max retries exceeded",
		},
		{
			name:     "unknown error returns message",
			text:     "custom error",
			expected: "custom error",
		},
		{
			name:     "wrapped max retries",
			text:     fmt.Errorf("failed: %w", contracts.ErrMaxRetriesExceeded).Error(),
			expected: "max retries exceeded",
		},
		{
			name:     "wrapped process exited",
			text:     fmt.Errorf("service api: %w", contracts.ErrProcessExited).Error(),
			expected: "process exited",
		},
		{
			name:     "wrapped readiness timeout",
			text:     fmt.Errorf("check failed: %w", contracts.ErrReadinessTimeout).Error(),
			expected: "readiness timeout",
		},
		{
			name:     "deeply wrapped error",
			text:     fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", contracts.ErrServiceNotFound)).Error(),
			expected: "service not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderError(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_GetUptime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		service *model.Service
		want    string
	}{
		{
			name:    "stopped service returns empty",
			service: &model.Service{Status: model.StatusStopped, Process: model.Process{StartedAt: now.Add(-1 * time.Hour)}},
			want:    "",
		},
		{
			name:    "failed service returns empty",
			service: &model.Service{Status: model.StatusFailed, Process: model.Process{StartedAt: now.Add(-1 * time.Hour)}},
			want:    "",
		},
		{
			name:    "zero start time returns empty",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{StartedAt: time.Time{}}},
			want:    "",
		},
		{
			name:    "seconds only",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{StartedAt: now.Add(-30 * time.Second)}},
			want:    "00:30",
		},
		{
			name:    "minutes and seconds",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{StartedAt: now.Add(-5*time.Minute - 45*time.Second)}},
			want:    "05:45",
		},
		{
			name:    "hours minutes seconds",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{StartedAt: now.Add(-2*time.Hour - 30*time.Minute - 15*time.Second)}},
			want:    "02:30:15",
		},
	}

	m := Model{}
	m.state.now = now

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.getUptime(tt.service)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_GetUptime_ZeroNow(t *testing.T) {
	m := Model{}
	service := &model.Service{Status: model.StatusRunning, Process: model.Process{StartedAt: time.Now().Add(-1 * time.Hour)}}

	assert.Empty(t, m.getUptime(service))
}

func Test_GetCPU(t *testing.T) {
	tests := []struct {
		name    string
		service *model.Service
		want    string
	}{
		{
			name:    "stopped service returns empty",
			service: &model.Service{Status: model.StatusStopped, Process: model.Process{PID: 1234, CPU: 50.0}},
			want:    "",
		},
		{
			name:    "zero PID returns empty",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 0, CPU: 50.0}},
			want:    "",
		},
		{
			name:    "formats CPU percentage",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 1234, CPU: 25.5}},
			want:    "25.5%",
		},
		{
			name:    "zero CPU",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 1234, CPU: 0.0}},
			want:    "0.0%",
		},
		{
			name:    "high CPU",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 1234, CPU: 99.9}},
			want:    "99.9%",
		},
	}

	m := Model{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.getCPU(tt.service)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_GetMem(t *testing.T) {
	tests := []struct {
		name    string
		service *model.Service
		want    string
	}{
		{
			name:    "stopped service returns empty",
			service: &model.Service{Status: model.StatusStopped, Process: model.Process{PID: 1234, Memory: 536870912}},
			want:    "",
		},
		{
			name:    "zero PID returns empty",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 0, Memory: 536870912}},
			want:    "",
		},
		{
			name:    "formats MB",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 1234, Memory: 268435456}},
			want:    "256MB",
		},
		{
			name:    "formats MB with decimal truncation",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 1234, Memory: 269169459}},
			want:    "257MB",
		},
		{
			name:    "formats GB for 1024MB or more",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 1234, Memory: 1073741824}},
			want:    "1.0GB",
		},
		{
			name:    "formats GB with decimal",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 1234, Memory: 2684354560}},
			want:    "2.5GB",
		},
		{
			name:    "small memory",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 1234, Memory: 2097152}},
			want:    "2MB",
		},
	}

	m := Model{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.getMem(tt.service)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_GetPID(t *testing.T) {
	tests := []struct {
		name    string
		service *model.Service
		want    string
	}{
		{
			name:    "stopped service returns empty",
			service: &model.Service{Status: model.StatusStopped, Process: model.Process{PID: 1234}},
			want:    "",
		},
		{
			name:    "zero PID returns empty",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 0}},
			want:    "",
		},
		{
			name:    "formats PID",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 1234}},
			want:    "1234",
		},
		{
			name:    "large PID",
			service: &model.Service{Status: model.StatusRunning, Process: model.Process{PID: 99999}},
			want:    "99999",
		},
	}

	m := Model{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.getPID(tt.service)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_Pad(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{
			input: 0,
			want:  "00",
		},
		{
			input: 1,
			want:  "01",
		},
		{
			input: 9,
			want:  "09",
		},
		{
			input: 10,
			want:  "10",
		},
		{
			input: 59,
			want:  "59",
		},
		{
			input: 100,
			want:  "100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, pad(tt.input))
		})
	}
}

func Test_FormatCPU(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  string
	}{
		{
			name:  "zero",
			input: 0,
			want:  "0.0%",
		},
		{
			name:  "fractional",
			input: 5.3,
			want:  "5.3%",
		},
		{
			name:  "high usage",
			input: 99.9,
			want:  "99.9%",
		},
		{
			name:  "over 100",
			input: 150.5,
			want:  "150.5%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatCPU(tt.input))
		})
	}
}

func Test_FormatMEM(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  string
	}{
		{
			name:  "zero",
			input: 0,
			want:  "0MB",
		},
		{
			name:  "small MB",
			input: 50,
			want:  "50MB",
		},
		{
			name:  "just below 1GB",
			input: 1023,
			want:  "1023MB",
		},
		{
			name:  "exactly 1GB",
			input: 1024,
			want:  "1.0GB",
		},
		{
			name:  "above 1GB",
			input: 2560,
			want:  "2.5GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatMEM(tt.input))
		})
	}
}
