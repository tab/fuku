package services

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/runtime"
)

func Test_NewController(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := runtime.NewMockCommandBus(ctrl)
	c := NewController(mockCmd)

	assert.NotNil(t, c)
	impl, ok := c.(*controller)
	assert.True(t, ok)
	assert.Equal(t, mockCmd, impl.command)
}

func Test_Controller_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := runtime.NewMockCommandBus(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	c := NewController(mockCmd)
	ctx := context.Background()

	tests := []struct {
		name           string
		service        *ServiceState
		before         func()
		expectedStatus Status
	}{
		{
			name:    "nil service",
			service: nil,
			before:  func() {},
		},
		{
			name:           "nil FSM",
			service:        &ServiceState{Name: "api", Status: StatusStopped},
			before:         func() {},
			expectedStatus: StatusStopped,
		},
		{
			name: "service not stopped",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusReady}
				s.FSM = newServiceFSM(s, loader, mockCmd)
				_ = s.FSM.Event(ctx, Start)
				_ = s.FSM.Event(ctx, Started)

				return s
			}(),
			before:         func() {},
			expectedStatus: StatusReady,
		},
		{
			name: "service stopped - starts successfully",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusStopped}
				s.FSM = newServiceFSM(s, loader, mockCmd)

				return s
			}(),
			before: func() {
				mockCmd.EXPECT().Publish(runtime.Command{
					Type: runtime.CommandRestartService,
					Data: runtime.RestartServiceData{Service: "api"},
				})
			},
			expectedStatus: StatusStarting,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()
			c.Start(ctx, tt.service)

			if tt.service != nil && tt.expectedStatus != "" {
				assert.Equal(t, tt.expectedStatus, tt.service.Status)
			}
		})
	}
}

func Test_Controller_Stop(t *testing.T) {
	noopCmd := runtime.NewNoOpCommandBus()
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	c := NewController(noopCmd)
	ctx := context.Background()

	tests := []struct {
		name           string
		service        *ServiceState
		expectedStatus Status
	}{
		{name: "nil service", service: nil},
		{name: "nil FSM", service: &ServiceState{Name: "api", Status: StatusReady}, expectedStatus: StatusReady},
		{
			name: "service not running",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusStopped}
				s.FSM = newServiceFSM(s, loader, noopCmd)

				return s
			}(),
			expectedStatus: StatusStopped,
		},
		{
			name: "service running - stops successfully",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusReady}
				s.FSM = newServiceFSM(s, loader, noopCmd)
				_ = s.FSM.Event(ctx, Start)
				_ = s.FSM.Event(ctx, Started)

				return s
			}(),
			expectedStatus: StatusStopping,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.Stop(ctx, tt.service)

			if tt.service != nil && tt.expectedStatus != "" {
				assert.Equal(t, tt.expectedStatus, tt.service.Status)
			}
		})
	}
}

func Test_Controller_Restart(t *testing.T) {
	noopCmd := runtime.NewNoOpCommandBus()
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	c := NewController(noopCmd)
	ctx := context.Background()

	tests := []struct {
		name           string
		service        *ServiceState
		expectedStatus Status
	}{
		{name: "nil service", service: nil},
		{name: "nil FSM", service: &ServiceState{Name: "api", Status: StatusReady}, expectedStatus: StatusReady},
		{
			name: "service starting - cannot restart",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusStarting}
				s.FSM = newServiceFSM(s, loader, noopCmd)
				_ = s.FSM.Event(ctx, Start)

				return s
			}(),
			expectedStatus: StatusStarting,
		},
		{
			name: "service running - restarts",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusReady}
				s.FSM = newServiceFSM(s, loader, noopCmd)
				_ = s.FSM.Event(ctx, Start)
				_ = s.FSM.Event(ctx, Started)

				return s
			}(),
			expectedStatus: StatusReady,
		},
		{
			name: "service failed - restarts",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusFailed}
				s.FSM = newServiceFSM(s, loader, noopCmd)
				_ = s.FSM.Event(ctx, Start)
				_ = s.FSM.Event(ctx, Failed)

				return s
			}(),
			expectedStatus: StatusFailed,
		},
		{
			name: "service stopped - restarts",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusStopped}
				s.FSM = newServiceFSM(s, loader, noopCmd)

				return s
			}(),
			expectedStatus: StatusStopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.Restart(ctx, tt.service)

			if tt.service != nil && tt.expectedStatus != "" {
				assert.Equal(t, tt.expectedStatus, tt.service.Status)
			}
		})
	}
}

func Test_Controller_HandleStarting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := runtime.NewMockCommandBus(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	c := NewController(mockCmd)
	ctx := context.Background()

	tests := []struct {
		name           string
		service        *ServiceState
		pid            int
		expectedPID    int
		expectedStatus Status
	}{
		{name: "nil service", service: nil, pid: 1234},
		{
			name:           "nil FSM - sets PID only",
			service:        &ServiceState{Name: "api", Status: StatusStarting},
			pid:            1234,
			expectedPID:    1234,
			expectedStatus: StatusStarting,
		},
		{
			name: "with FSM - sets PID and transitions",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusStopped}
				s.FSM = newServiceFSM(s, loader, mockCmd)

				return s
			}(),
			pid:            5678,
			expectedPID:    5678,
			expectedStatus: StatusStarting,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.HandleStarting(ctx, tt.service, tt.pid)

			if tt.service != nil {
				assert.Equal(t, tt.expectedPID, tt.service.Monitor.PID)

				if tt.expectedStatus != "" {
					assert.Equal(t, tt.expectedStatus, tt.service.Status)
				}
			}
		})
	}
}

func Test_Controller_HandleReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := runtime.NewMockCommandBus(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	c := NewController(mockCmd)
	ctx := context.Background()

	tests := []struct {
		name           string
		service        *ServiceState
		expectedStatus Status
	}{
		{name: "nil service", service: nil},
		{name: "nil FSM", service: &ServiceState{Name: "api", Status: StatusStarting}, expectedStatus: StatusStarting},
		{
			name: "with FSM - transitions to ready",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusStarting}
				s.FSM = newServiceFSM(s, loader, mockCmd)
				_ = s.FSM.Event(ctx, Start)

				return s
			}(),
			expectedStatus: StatusReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.HandleReady(ctx, tt.service)

			if tt.service != nil && tt.expectedStatus != "" {
				assert.Equal(t, tt.expectedStatus, tt.service.Status)
			}
		})
	}
}

func Test_Controller_HandleFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := runtime.NewMockCommandBus(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	c := NewController(mockCmd)
	ctx := context.Background()

	tests := []struct {
		name           string
		service        *ServiceState
		expectedStatus Status
	}{
		{name: "nil service", service: nil},
		{name: "nil FSM", service: &ServiceState{Name: "api", Status: StatusStarting}, expectedStatus: StatusStarting},
		{
			name: "with FSM - transitions to failed",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusStarting}
				s.FSM = newServiceFSM(s, loader, mockCmd)
				_ = s.FSM.Event(ctx, Start)

				return s
			}(),
			expectedStatus: StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.HandleFailed(ctx, tt.service)

			if tt.service != nil && tt.expectedStatus != "" {
				assert.Equal(t, tt.expectedStatus, tt.service.Status)
			}
		})
	}
}

func Test_Controller_HandleStopped(t *testing.T) {
	noopCmd := runtime.NewNoOpCommandBus()
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	c := NewController(noopCmd)
	ctx := context.Background()

	tests := []struct {
		name           string
		service        *ServiceState
		expectedResult bool
		expectedStatus Status
		expectedPID    int
		expectedCPU    float64
		expectedMEM    float64
	}{
		{name: "nil service", service: nil, expectedResult: false},
		{
			name:           "nil FSM - marks stopped",
			service:        &ServiceState{Name: "api", Status: StatusReady, Monitor: ServiceMonitor{PID: 1234, CPU: 10.5, MEM: 1000}},
			expectedResult: false,
			expectedStatus: StatusStopped,
			expectedPID:    0,
			expectedCPU:    10.5,
			expectedMEM:    1000,
		},
		{
			name: "FSM in stopping state - transitions to stopped",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusReady, Monitor: ServiceMonitor{PID: 1234}}
				s.FSM = newServiceFSM(s, loader, noopCmd)
				_ = s.FSM.Event(ctx, Start)
				_ = s.FSM.Event(ctx, Started)
				_ = s.FSM.Event(ctx, Stop)

				return s
			}(),
			expectedResult: false,
			expectedStatus: StatusStopped,
			expectedPID:    0,
		},
		{
			name: "FSM in restarting state - returns true",
			service: func() *ServiceState {
				s := &ServiceState{Name: "api", Status: StatusReady, Monitor: ServiceMonitor{PID: 1234}}
				s.FSM = newServiceFSM(s, loader, noopCmd)
				_ = s.FSM.Event(ctx, Start)
				_ = s.FSM.Event(ctx, Started)
				_ = s.FSM.Event(ctx, Restart)

				return s
			}(),
			expectedResult: true,
			expectedStatus: StatusStopped,
			expectedPID:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.HandleStopped(ctx, tt.service)
			assert.Equal(t, tt.expectedResult, result)

			if tt.service != nil {
				if tt.expectedStatus != "" {
					assert.Equal(t, tt.expectedStatus, tt.service.Status)
				}

				assert.Equal(t, tt.expectedPID, tt.service.Monitor.PID)
				assert.Equal(t, tt.expectedCPU, tt.service.Monitor.CPU)
				assert.Equal(t, tt.expectedMEM, tt.service.Monitor.MEM)
				assert.Nil(t, tt.service.Error)
			}
		})
	}
}
