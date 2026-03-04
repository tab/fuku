package services

import (
	"context"
	"testing"
	"time"

	"github.com/looplab/fsm"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/monitor"
	"fuku/internal/app/ui/components"
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
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{StartTime: time.Time{}}},
			want:    "",
		},
		{
			name:    "seconds only",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{StartTime: now.Add(-30 * time.Second)}},
			want:    "00:30",
		},
		{
			name:    "minutes and seconds",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{StartTime: now.Add(-5*time.Minute - 45*time.Second)}},
			want:    "05:45",
		},
		{
			name:    "hours minutes seconds",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{StartTime: now.Add(-2*time.Hour - 30*time.Minute - 15*time.Second)}},
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
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 0, CPU: 50.0}},
			want:    "",
		},
		{
			name:    "formats CPU percentage",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 1234, CPU: 25.5}},
			want:    "25.5%",
		},
		{
			name:    "zero CPU",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 1234, CPU: 0.0}},
			want:    "0.0%",
		},
		{
			name:    "high CPU",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 1234, CPU: 99.9}},
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
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 0, MEM: 512.0}},
			want:    "",
		},
		{
			name:    "formats MB",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 1234, MEM: 256.0}},
			want:    "256MB",
		},
		{
			name:    "formats MB with decimal truncation",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 1234, MEM: 256.7}},
			want:    "257MB",
		},
		{
			name:    "formats GB for 1024MB or more",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 1234, MEM: 1024.0}},
			want:    "1.0GB",
		},
		{
			name:    "formats GB with decimal",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 1234, MEM: 2560.0}},
			want:    "2.5GB",
		},
		{
			name:    "small memory",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 1234, MEM: 2.0}},
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
		service *ServiceState
		want    string
	}{
		{
			name:    "stopped service returns empty",
			service: &ServiceState{Status: StatusStopped, Monitor: ServiceMonitor{PID: 1234}},
			want:    "",
		},
		{
			name:    "zero PID returns empty",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 0}},
			want:    "",
		},
		{
			name:    "formats PID",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 1234}},
			want:    "1234",
		},
		{
			name:    "large PID",
			service: &ServiceState{Status: StatusRunning, Monitor: ServiceMonitor{PID: 99999}},
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

func Test_ApplyStatsUpdate(t *testing.T) {
	m := &Model{}
	m.state.services = map[string]*ServiceState{
		"api":      {Name: "api", Monitor: ServiceMonitor{PID: 1234}},
		"database": {Name: "database", Monitor: ServiceMonitor{PID: 5678}},
	}

	msg := statsUpdateMsg{
		Stats: map[string]ServiceStats{
			"api":      {CPU: 25.5, MEM: 512.0},
			"database": {CPU: 10.0, MEM: 1024.0},
			"unknown":  {CPU: 99.0, MEM: 999.0},
		},
	}

	m.applyStatsUpdate(msg)

	assert.Equal(t, 25.5, m.state.services["api"].Monitor.CPU)
	assert.Equal(t, 512.0, m.state.services["api"].Monitor.MEM)
	assert.Equal(t, 10.0, m.state.services["database"].Monitor.CPU)
	assert.Equal(t, 1024.0, m.state.services["database"].Monitor.MEM)
}

func Test_ApplyStatsUpdate_EmptyServices(t *testing.T) {
	m := &Model{}
	m.state.services = map[string]*ServiceState{}

	msg := statsUpdateMsg{
		Stats: map[string]ServiceStats{
			"api": {CPU: 25.5, MEM: 512.0},
		},
	}

	m.applyStatsUpdate(msg)

	assert.Empty(t, m.state.services)
}

func Test_UpdateBlinkAnimations(t *testing.T) {
	tests := []struct {
		name               string
		services           map[string]*ServiceState
		expectedHasActive  bool
		expectBlinkStarted map[string]bool
	}{
		{
			name:              "no services",
			services:          map[string]*ServiceState{},
			expectedHasActive: false,
		},
		{
			name: "service with nil blink",
			services: map[string]*ServiceState{
				"api": {Name: "api", Blink: nil},
			},
			expectedHasActive: false,
		},
		{
			name: "service with nil FSM",
			services: map[string]*ServiceState{
				"api": {Name: "api", Blink: components.NewBlink(), FSM: nil},
			},
			expectedHasActive: false,
		},
		{
			name: "running service stops blink",
			services: map[string]*ServiceState{
				"api": func() *ServiceState {
					s := &ServiceState{Name: "api", Blink: components.NewBlink()}
					s.Blink.Start()
					s.FSM = newTestFSM(Running)

					return s
				}(),
			},
			expectedHasActive: false,
		},
		{
			name: "starting service activates blink",
			services: map[string]*ServiceState{
				"api": func() *ServiceState {
					s := &ServiceState{Name: "api", Blink: components.NewBlink()}
					s.FSM = newTestFSM(Starting)

					return s
				}(),
			},
			expectedHasActive:  true,
			expectBlinkStarted: map[string]bool{"api": true},
		},
		{
			name: "stopping service activates blink",
			services: map[string]*ServiceState{
				"api": func() *ServiceState {
					s := &ServiceState{Name: "api", Blink: components.NewBlink()}
					s.FSM = newTestFSM(Stopping)

					return s
				}(),
			},
			expectedHasActive:  true,
			expectBlinkStarted: map[string]bool{"api": true},
		},
		{
			name: "restarting service activates blink",
			services: map[string]*ServiceState{
				"api": func() *ServiceState {
					s := &ServiceState{Name: "api", Blink: components.NewBlink()}
					s.FSM = newTestFSM(Restarting)

					return s
				}(),
			},
			expectedHasActive:  true,
			expectBlinkStarted: map[string]bool{"api": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{}
			m.state.services = tt.services

			result := m.updateBlinkAnimations()

			assert.Equal(t, tt.expectedHasActive, result)

			for name, expectStarted := range tt.expectBlinkStarted {
				assert.Equal(t, expectStarted, m.state.services[name].Blink.IsActive())
			}
		})
	}
}

func newTestFSM(initialState string) *fsm.FSM {
	return fsm.NewFSM(initialState, fsm.Events{}, fsm.Callbacks{})
}

func Test_CollectStats_NoMonitoredServices(t *testing.T) {
	ctx := context.Background()
	stats, nextOffset := collectStats(ctx, nil, nil, 0)

	assert.Nil(t, stats)
	assert.Equal(t, 0, nextOffset)
}

func Test_CollectStats_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockMonitor.EXPECT().GetStats(gomock.Any(), 1234).Return(monitor.Stats{CPU: 25.5, MEM: 512.0}, nil)
	mockMonitor.EXPECT().GetStats(gomock.Any(), 5678).Return(monitor.Stats{CPU: 10.0, MEM: 256.0}, nil)

	services := []monitoredService{
		{name: "api", pid: 1234},
		{name: "database", pid: 5678},
	}

	ctx := context.Background()
	stats, _ := collectStats(ctx, mockMonitor, services, 0)

	assert.Len(t, stats, 2)
	assert.Equal(t, 25.5, stats["api"].CPU)
	assert.Equal(t, 512.0, stats["api"].MEM)
	assert.Equal(t, 10.0, stats["database"].CPU)
	assert.Equal(t, 256.0, stats["database"].MEM)
}

func Test_CollectStats_ContextCancelled(t *testing.T) {
	services := []monitoredService{
		{name: "api", pid: 1234},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stats, _ := collectStats(ctx, nil, services, 0)

	assert.Empty(t, stats)
}

func Test_CollectStats_MonitorError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockMonitor.EXPECT().GetStats(gomock.Any(), 1234).Return(monitor.Stats{}, assert.AnError)

	services := []monitoredService{
		{name: "api", pid: 1234},
	}

	ctx := context.Background()
	stats, _ := collectStats(ctx, mockMonitor, services, 0)

	assert.Empty(t, stats)
}

func Test_CollectStats_RoundRobin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMonitor := monitor.NewMockMonitor(ctrl)
	first := mockMonitor.EXPECT().GetStats(gomock.Any(), 5678).Return(monitor.Stats{CPU: 10.0, MEM: 256.0}, nil)
	mockMonitor.EXPECT().GetStats(gomock.Any(), 1234).Return(monitor.Stats{CPU: 25.5, MEM: 512.0}, nil).After(first)

	services := []monitoredService{
		{name: "api", pid: 1234},
		{name: "database", pid: 5678},
	}

	ctx := context.Background()
	stats, nextOffset := collectStats(ctx, mockMonitor, services, 1)

	assert.Len(t, stats, 2)
	assert.Equal(t, 25.5, stats["api"].CPU)
	assert.Equal(t, 10.0, stats["database"].CPU)
	assert.Equal(t, 1, nextOffset)
}

func Test_CollectStats_OffsetWraps(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockMonitor.EXPECT().GetStats(gomock.Any(), gomock.Any()).Return(monitor.Stats{CPU: 1.0, MEM: 10.0}, nil).Times(2)

	services := []monitoredService{
		{name: "api", pid: 1234},
		{name: "database", pid: 5678},
	}

	ctx := context.Background()
	_, nextOffset := collectStats(ctx, mockMonitor, services, 99)

	assert.Equal(t, 0, nextOffset)
}

func Test_CollectStats_BatchBudgetExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())

	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockMonitor.EXPECT().GetStats(gomock.Any(), 1234).DoAndReturn(
		func(_ context.Context, _ int) (monitor.Stats, error) {
			cancel()

			return monitor.Stats{CPU: 25.5, MEM: 512.0}, nil
		},
	)

	services := []monitoredService{
		{name: "api", pid: 1234},
		{name: "database", pid: 5678},
		{name: "cache", pid: 9012},
	}

	stats, nextOffset := collectStats(ctx, mockMonitor, services, 0)

	assert.Contains(t, stats, "api")
	assert.Len(t, stats, 1)
	assert.Equal(t, 1, nextOffset)
}

func Test_CollectStats_BatchBudgetExpiredDuringCall(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())

	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockMonitor.EXPECT().GetStats(gomock.Any(), 5678).DoAndReturn(
		func(_ context.Context, _ int) (monitor.Stats, error) {
			cancel()

			return monitor.Stats{}, context.Canceled
		},
	)

	services := []monitoredService{
		{name: "api", pid: 1234},
		{name: "database", pid: 5678},
		{name: "cache", pid: 9012},
	}

	stats, nextOffset := collectStats(ctx, mockMonitor, services, 1)

	assert.Empty(t, stats)
	assert.Equal(t, 1, nextOffset)
}

func Test_StatsWorkerCmd_ReturnsCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockMonitor.EXPECT().GetStats(gomock.Any(), gomock.Any()).Return(monitor.Stats{CPU: 25.5, MEM: 512.0}, nil).AnyTimes()

	services := []monitoredService{
		{name: "api", pid: 1234},
	}

	ctx := context.Background()
	cmd := statsWorkerCmd(ctx, mockMonitor, services, 0)

	assert.NotNil(t, cmd)

	msg := cmd()
	assert.NotNil(t, msg)
}

func Test_BuildMonitoredList(t *testing.T) {
	m := &Model{}
	m.state.tiers = []Tier{
		{Name: "foundation", Services: []string{"db", "cache"}},
		{Name: "app", Services: []string{"api", "web"}},
	}
	m.state.services = map[string]*ServiceState{
		"db":    {Name: "db", Status: StatusRunning, Monitor: ServiceMonitor{PID: 100}},
		"cache": {Name: "cache", Status: StatusStopped, Monitor: ServiceMonitor{PID: 0}},
		"api":   {Name: "api", Status: StatusRunning, Monitor: ServiceMonitor{PID: 200}},
		"web":   {Name: "web", Status: StatusRunning, Monitor: ServiceMonitor{PID: 300}},
	}

	list := m.buildMonitoredList()

	assert.Len(t, list, 3)
	assert.Equal(t, "db", list[0].name)
	assert.Equal(t, 100, list[0].pid)
	assert.Equal(t, "api", list[1].name)
	assert.Equal(t, 200, list[1].pid)
	assert.Equal(t, "web", list[2].name)
	assert.Equal(t, 300, list[2].pid)
}
