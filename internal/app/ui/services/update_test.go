package services

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/runtime"
	"fuku/internal/config/logger"
)

func newTestLogger(ctrl *gomock.Controller) *logger.MockLogger {
	mockLog := logger.NewMockLogger(ctrl)
	noopLogger := zerolog.New(io.Discard)
	noopEvent := noopLogger.Debug()
	mockLog.EXPECT().Debug().Return(noopEvent).AnyTimes()
	mockLog.EXPECT().Info().Return(noopEvent).AnyTimes()
	mockLog.EXPECT().Warn().Return(noopEvent).AnyTimes()
	mockLog.EXPECT().Error().Return(noopEvent).AnyTimes()

	return mockLog
}

func Test_HandleProfileResolved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := Model{
		services: make(map[string]*ServiceState),
		tiers:    make([]TierView, 0),
		log:      newTestLogger(ctrl),
	}

	event := runtime.Event{
		Type: runtime.EventProfileResolved,
		Data: runtime.ProfileResolvedData{
			Profile: "dev",
			Tiers: []runtime.TierData{
				{Name: "tier1", Services: []string{"db"}},
				{Name: "tier2", Services: []string{"api", "web"}},
			},
		},
	}

	result := m.handleProfileResolved(event)

	assert.Len(t, result.tiers, 2)
	assert.Equal(t, "tier1", result.tiers[0].Name)
	assert.Equal(t, "tier2", result.tiers[1].Name)
	assert.Equal(t, []string{"db"}, result.tiers[0].Services)
	assert.Equal(t, []string{"api", "web"}, result.tiers[1].Services)
	assert.Len(t, result.services, 3)
	assert.NotNil(t, result.services["db"])
	assert.NotNil(t, result.services["api"])
	assert.NotNil(t, result.services["web"])
	assert.Equal(t, StatusStarting, result.services["db"].Status)
}

func Test_HandleProfileResolved_InvalidData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := Model{services: make(map[string]*ServiceState), tiers: make([]TierView, 0), log: newTestLogger(ctrl)}
	event := runtime.Event{Type: runtime.EventProfileResolved, Data: "invalid"}
	result := m.handleProfileResolved(event)
	assert.Len(t, result.tiers, 0)
}

func Test_HandleTierStarting(t *testing.T) {
	m := Model{
		tiers: []TierView{
			{Name: "tier1", Ready: true},
			{Name: "tier2", Ready: true},
		},
	}

	event := runtime.Event{Type: runtime.EventTierStarting, Data: runtime.TierStartingData{Name: "tier1"}}
	result := m.handleTierStarting(event)

	assert.False(t, result.tiers[0].Ready)
	assert.True(t, result.tiers[1].Ready)
}

func Test_HandleTierStarting_InvalidData(t *testing.T) {
	m := Model{tiers: []TierView{{Name: "tier1", Ready: true}}}
	event := runtime.Event{Type: runtime.EventTierStarting, Data: "invalid"}
	result := m.handleTierStarting(event)
	assert.True(t, result.tiers[0].Ready)
}

func Test_HandleTierReady(t *testing.T) {
	m := Model{
		tiers: []TierView{
			{Name: "tier1", Ready: false},
			{Name: "tier2", Ready: false},
		},
	}

	event := runtime.Event{Type: runtime.EventTierReady, Data: runtime.TierReadyData{Name: "tier2"}}
	result := m.handleTierReady(event)

	assert.False(t, result.tiers[0].Ready)
	assert.True(t, result.tiers[1].Ready)
}

func Test_HandleTierReady_InvalidData(t *testing.T) {
	m := Model{tiers: []TierView{{Name: "tier1", Ready: false}}}
	event := runtime.Event{Type: runtime.EventTierReady, Data: "invalid"}
	result := m.handleTierReady(event)
	assert.False(t, result.tiers[0].Ready)
}

func Test_HandleServiceStarting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := runtime.NewMockCommandBus(ctrl)
	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	service := &ServiceState{Name: "api", Status: StatusStopped}
	m := Model{
		ctx:        context.Background(),
		services:   map[string]*ServiceState{"api": service},
		loader:     loader,
		command:    mockCmd,
		controller: mockController,
	}
	service.FSM = newServiceFSM(service, loader)

	mockController.EXPECT().HandleStarting(gomock.Any(), service, 1234).Do(
		func(_ context.Context, s *ServiceState, pid int) {
			s.Monitor.PID = pid
		},
	)

	event := runtime.Event{
		Timestamp: time.Now(),
		Type:      runtime.EventServiceStarting,
		Data:      runtime.ServiceStartingData{Service: "api", Tier: "tier1", PID: 1234},
	}

	result := m.handleServiceStarting(event)

	assert.Equal(t, "tier1", result.services["api"].Tier)
	assert.Equal(t, 1234, result.services["api"].Monitor.PID)
}

func Test_HandleServiceStarting_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{services: make(map[string]*ServiceState), loader: loader}
	event := runtime.Event{Type: runtime.EventServiceStarting, Data: "invalid"}
	result := m.handleServiceStarting(event)
	assert.False(t, result.loader.Active)
}

func Test_HandleServiceReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("api", "Starting api…")

	service := &ServiceState{Name: "api", Status: StatusStarting}
	m := Model{
		ctx:        context.Background(),
		services:   map[string]*ServiceState{"api": service},
		loader:     loader,
		controller: mockController,
	}

	mockController.EXPECT().HandleReady(gomock.Any(), service)

	event := runtime.Event{
		Timestamp: time.Now(),
		Type:      runtime.EventServiceReady,
		Data:      runtime.ServiceReadyData{Service: "api"},
	}

	result := m.handleServiceReady(event)

	assert.False(t, result.loader.Active)
	assert.NotZero(t, result.services["api"].Monitor.ReadyTime)
}

func Test_HandleServiceReady_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("api", "Starting api…")
	m := Model{services: make(map[string]*ServiceState), loader: loader}
	event := runtime.Event{Type: runtime.EventServiceReady, Data: "invalid"}
	result := m.handleServiceReady(event)
	assert.True(t, result.loader.Active)
}

func Test_HandleServiceFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("api", "Starting api…")

	service := &ServiceState{Name: "api", Status: StatusStarting}
	m := Model{
		ctx:        context.Background(),
		services:   map[string]*ServiceState{"api": service},
		loader:     loader,
		controller: mockController,
	}

	mockController.EXPECT().HandleFailed(gomock.Any(), service)

	testErr := assert.AnError
	event := runtime.Event{
		Type: runtime.EventServiceFailed,
		Data: runtime.ServiceFailedData{Service: "api", Error: testErr},
	}

	result := m.handleServiceFailed(event)

	assert.False(t, result.loader.Active)
	assert.Equal(t, testErr, result.services["api"].Error)
}

func Test_HandleServiceFailed_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{services: make(map[string]*ServiceState), loader: loader}
	event := runtime.Event{Type: runtime.EventServiceFailed, Data: "invalid"}
	result := m.handleServiceFailed(event)
	assert.Len(t, result.services, 0)
}

func Test_HandleServiceStopped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("api", "Stopping api…")

	service := &ServiceState{Name: "api", Status: StatusReady, Monitor: ServiceMonitor{PID: 1234}}
	m := Model{
		ctx:        context.Background(),
		services:   map[string]*ServiceState{"api": service},
		loader:     loader,
		controller: mockController,
	}

	mockController.EXPECT().HandleStopped(gomock.Any(), service).Return(false)

	event := runtime.Event{Type: runtime.EventServiceStopped, Data: runtime.ServiceStoppedData{Service: "api"}}
	result := m.handleServiceStopped(event)

	assert.False(t, result.loader.Active)
}

func Test_HandleServiceStopped_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{services: make(map[string]*ServiceState), loader: loader}
	event := runtime.Event{Type: runtime.EventServiceStopped, Data: "invalid"}
	result := m.handleServiceStopped(event)
	assert.Len(t, result.services, 0)
}

func Test_HandleLogLine(t *testing.T) {
	service := &ServiceState{Name: "api", LogEnabled: true}
	m := Model{
		services: map[string]*ServiceState{"api": service},
		logs:     make([]LogEntry, 0),
		maxLogs:  1000,
	}

	event := runtime.Event{
		Timestamp: time.Now(),
		Type:      runtime.EventLogLine,
		Data:      runtime.LogLineData{Service: "api", Tier: "tier1", Stream: "STDOUT", Message: "Server started"},
	}

	result := m.handleLogLine(event)

	assert.Len(t, result.logs, 1)
	assert.Equal(t, "api", result.logs[0].Service)
	assert.Equal(t, "tier1", result.logs[0].Tier)
	assert.Equal(t, "STDOUT", result.logs[0].Stream)
	assert.Equal(t, "Server started", result.logs[0].Message)
}

func Test_HandleLogLine_TruncatesOldLogs(t *testing.T) {
	service := &ServiceState{Name: "api", LogEnabled: true}
	m := Model{
		services: map[string]*ServiceState{"api": service},
		logs:     make([]LogEntry, 10),
		maxLogs:  10,
	}

	event := runtime.Event{
		Timestamp: time.Now(),
		Type:      runtime.EventLogLine,
		Data:      runtime.LogLineData{Service: "api", Tier: "tier1", Stream: "STDOUT", Message: "New log"},
	}

	result := m.handleLogLine(event)

	assert.Len(t, result.logs, 10)
	assert.Equal(t, "New log", result.logs[9].Message)
}

func Test_HandleLogLine_InvalidData(t *testing.T) {
	m := Model{logs: make([]LogEntry, 0)}
	event := runtime.Event{Type: runtime.EventLogLine, Data: "invalid"}
	result := m.handleLogLine(event)
	assert.Len(t, result.logs, 0)
}
