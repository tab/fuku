package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/registry"
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
			service: &ServiceState{Status: StatusStopped, StartTime: now.Add(-1 * time.Hour)},
			want:    "",
		},
		{
			name:    "failed service returns empty",
			service: &ServiceState{Status: StatusFailed, StartTime: now.Add(-1 * time.Hour)},
			want:    "",
		},
		{
			name:    "zero start time returns empty",
			service: &ServiceState{Status: StatusRunning, StartTime: time.Time{}},
			want:    "",
		},
		{
			name:    "seconds only",
			service: &ServiceState{Status: StatusRunning, StartTime: now.Add(-30 * time.Second)},
			want:    "00:30",
		},
		{
			name:    "minutes and seconds",
			service: &ServiceState{Status: StatusRunning, StartTime: now.Add(-5*time.Minute - 45*time.Second)},
			want:    "05:45",
		},
		{
			name:    "hours minutes seconds",
			service: &ServiceState{Status: StatusRunning, StartTime: now.Add(-2*time.Hour - 30*time.Minute - 15*time.Second)},
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
	service := &ServiceState{Status: StatusRunning, StartTime: time.Now().Add(-1 * time.Hour)}

	assert.Empty(t, m.getUptime(service))
}

func Test_GetCPU(t *testing.T) {
	tests := []struct {
		name    string
		service *ServiceState
		want    string
	}{
		{
			name:    "stopped service returns empty",
			service: &ServiceState{Status: StatusStopped, PID: 1234, CPU: 50.0},
			want:    "",
		},
		{
			name:    "zero PID returns empty",
			service: &ServiceState{Status: StatusRunning, PID: 0, CPU: 50.0},
			want:    "",
		},
		{
			name:    "formats CPU percentage",
			service: &ServiceState{Status: StatusRunning, PID: 1234, CPU: 25.5},
			want:    "25.5%",
		},
		{
			name:    "zero CPU",
			service: &ServiceState{Status: StatusRunning, PID: 1234, CPU: 0.0},
			want:    "0.0%",
		},
		{
			name:    "high CPU",
			service: &ServiceState{Status: StatusRunning, PID: 1234, CPU: 99.9},
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
			service: &ServiceState{Status: StatusStopped, PID: 1234, MEM: 512.0},
			want:    "",
		},
		{
			name:    "zero PID returns empty",
			service: &ServiceState{Status: StatusRunning, PID: 0, MEM: 512.0},
			want:    "",
		},
		{
			name:    "formats MB",
			service: &ServiceState{Status: StatusRunning, PID: 1234, MEM: 256.0},
			want:    "256MB",
		},
		{
			name:    "formats MB with decimal truncation",
			service: &ServiceState{Status: StatusRunning, PID: 1234, MEM: 256.7},
			want:    "257MB",
		},
		{
			name:    "formats GB for 1024MB or more",
			service: &ServiceState{Status: StatusRunning, PID: 1234, MEM: 1024.0},
			want:    "1.0GB",
		},
		{
			name:    "formats GB with decimal",
			service: &ServiceState{Status: StatusRunning, PID: 1234, MEM: 2560.0},
			want:    "2.5GB",
		},
		{
			name:    "small memory",
			service: &ServiceState{Status: StatusRunning, PID: 1234, MEM: 2.0},
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
			service: &ServiceState{Status: StatusStopped, PID: 1234},
			want:    "",
		},
		{
			name:    "zero PID returns empty",
			service: &ServiceState{Status: StatusRunning, PID: 0},
			want:    "",
		},
		{
			name:    "formats PID",
			service: &ServiceState{Status: StatusRunning, PID: 1234},
			want:    "1234",
		},
		{
			name:    "large PID",
			service: &ServiceState{Status: StatusRunning, PID: 99999},
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
			name: "running service stops blink",
			services: map[string]*ServiceState{
				"api": func() *ServiceState {
					s := &ServiceState{Name: "api", Status: StatusRunning, Blink: components.NewBlink()}
					s.Blink.Start()

					return s
				}(),
			},
			expectedHasActive: false,
		},
		{
			name: "starting service activates blink",
			services: map[string]*ServiceState{
				"api": {
					Name:   "api",
					Status: StatusStarting,
					Blink:  components.NewBlink(),
				},
			},
			expectedHasActive:  true,
			expectBlinkStarted: map[string]bool{"api": true},
		},
		{
			name: "stopping service activates blink",
			services: map[string]*ServiceState{
				"api": {
					Name:   "api",
					Status: StatusStopping,
					Blink:  components.NewBlink(),
				},
			},
			expectedHasActive:  true,
			expectBlinkStarted: map[string]bool{"api": true},
		},
		{
			name: "restarting service activates blink",
			services: map[string]*ServiceState{
				"api": {
					Name:   "api",
					Status: StatusRestarting,
					Blink:  components.NewBlink(),
				},
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

func Test_UpdateAPIHealth(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := registry.NewMockStore(ctrl)

	tests := []struct {
		name       string
		apiAddr    string
		resolved   bool
		wantStatus apiHealthStatus
	}{
		{
			name:       "nil api returns down",
			wantStatus: apiStatusDown,
		},
		{
			name:       "empty address returns down",
			apiAddr:    "",
			wantStatus: apiStatusDown,
		},
		{
			name:       "address set but not resolved returns down",
			apiAddr:    "localhost:9876",
			resolved:   false,
			wantStatus: apiStatusDown,
		},
		{
			name:       "address set and resolved returns ready",
			apiAddr:    "localhost:9876",
			resolved:   true,
			wantStatus: apiStatusReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{store: mockStore}

			if tt.apiAddr != "" {
				m.api = &stubListener{addr: tt.apiAddr}
				mockStore.EXPECT().IsResolved().Return(tt.resolved)
			}

			m.updateAPIHealth()

			assert.Equal(t, tt.wantStatus, m.state.apiStatus)
		})
	}
}

type stubListener struct {
	addr string
}

func (s *stubListener) Address() string {
	return s.addr
}
