package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_GetUptime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		service *ServiceState
		want    string
	}{
		{
			name:    "stopped service returns empty",
			service: &ServiceState{Status: StatusStopped, Monitor: ServiceMonitor{StartTime: now.Add(-1 * time.Hour)}},
			want:    "",
		},
		{
			name:    "zero start time returns empty",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{StartTime: time.Time{}}},
			want:    "",
		},
		{
			name:    "seconds only",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{StartTime: now.Add(-30 * time.Second)}},
			want:    "00:30",
		},
		{
			name:    "minutes and seconds",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{StartTime: now.Add(-5*time.Minute - 45*time.Second)}},
			want:    "05:45",
		},
		{
			name:    "hours minutes seconds",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{StartTime: now.Add(-2*time.Hour - 30*time.Minute - 15*time.Second)}},
			want:    "02:30:15",
		},
	}

	m := Model{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.getUptime(tt.service)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_GetCPU(t *testing.T) {
	tests := []struct {
		name    string
		service *ServiceState
		want    string
	}{
		{
			name:    "stopped service returns empty",
			service: &ServiceState{Status: StatusStopped, Monitor: ServiceMonitor{PID: 1234, CPU: 50.0}},
			want:    "",
		},
		{
			name:    "zero PID returns empty",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{PID: 0, CPU: 50.0}},
			want:    "",
		},
		{
			name:    "formats CPU percentage",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{PID: 1234, CPU: 25.5}},
			want:    "25.5%",
		},
		{
			name:    "zero CPU",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{PID: 1234, CPU: 0.0}},
			want:    "0.0%",
		},
		{
			name:    "high CPU",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{PID: 1234, CPU: 99.9}},
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
		service *ServiceState
		want    string
	}{
		{
			name:    "stopped service returns empty",
			service: &ServiceState{Status: StatusStopped, Monitor: ServiceMonitor{PID: 1234, MEM: 512.0}},
			want:    "",
		},
		{
			name:    "zero PID returns empty",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{PID: 0, MEM: 512.0}},
			want:    "",
		},
		{
			name:    "formats MB",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{PID: 1234, MEM: 256.0}},
			want:    "256MB",
		},
		{
			name:    "formats MB with decimal truncation",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{PID: 1234, MEM: 256.7}},
			want:    "257MB",
		},
		{
			name:    "formats GB for 1024MB or more",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{PID: 1234, MEM: 1024.0}},
			want:    "1.0GB",
		},
		{
			name:    "formats GB with decimal",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{PID: 1234, MEM: 2560.0}},
			want:    "2.5GB",
		},
		{
			name:    "small memory",
			service: &ServiceState{Status: StatusReady, Monitor: ServiceMonitor{PID: 1234, MEM: 2.0}},
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

func Test_Pad(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "00"},
		{1, "01"},
		{9, "09"},
		{10, "10"},
		{59, "59"},
		{100, "100"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, pad(tt.input))
		})
	}
}
