package services

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
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

	m := Model{log: newTestLogger(ctrl), loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.tiers = make([]Tier, 0)

	event := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "dev",
			Tiers: []bus.Tier{
				{Name: "tier1", Services: []string{"db"}},
				{Name: "tier2", Services: []string{"api", "web"}},
			},
		},
	}

	result := m.handleProfileResolved(event)

	assert.Len(t, result.state.tiers, 2)
	assert.Equal(t, "tier1", result.state.tiers[0].Name)
	assert.Equal(t, "tier2", result.state.tiers[1].Name)
	assert.Equal(t, []string{"db"}, result.state.tiers[0].Services)
	assert.Equal(t, []string{"api", "web"}, result.state.tiers[1].Services)
	assert.Len(t, result.state.services, 3)
	assert.NotNil(t, result.state.services["db"])
	assert.NotNil(t, result.state.services["api"])
	assert.NotNil(t, result.state.services["web"])
	assert.Equal(t, StatusStarting, result.state.services["db"].Status)
}

func Test_HandleProfileResolved_InvalidData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := Model{log: newTestLogger(ctrl), loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.tiers = make([]Tier, 0)
	event := bus.Message{Type: bus.EventProfileResolved, Data: "invalid"}
	result := m.handleProfileResolved(event)
	assert.Len(t, result.state.tiers, 0)
}

func Test_HandleProfileResolved_ClearsStaleServices(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := Model{log: newTestLogger(ctrl), loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.tiers = make([]Tier, 0)

	event1 := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "profile1",
			Tiers: []bus.Tier{
				{Name: "tier1", Services: []string{"db", "api", "web"}},
			},
		},
	}

	result := m.handleProfileResolved(event1)
	assert.Len(t, result.state.services, 3, "First profile should have 3 services")
	assert.NotNil(t, result.state.services["db"])
	assert.NotNil(t, result.state.services["api"])
	assert.NotNil(t, result.state.services["web"])

	event2 := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "profile2",
			Tiers: []bus.Tier{
				{Name: "tier1", Services: []string{"storage", "cache"}},
			},
		},
	}

	result = result.handleProfileResolved(event2)
	assert.Len(t, result.state.services, 2, "Second profile should have exactly 2 services, not 5")
	assert.NotNil(t, result.state.services["storage"])
	assert.NotNil(t, result.state.services["cache"])
	assert.Nil(t, result.state.services["db"], "Old service 'db' should be removed")
	assert.Nil(t, result.state.services["api"], "Old service 'api' should be removed")
	assert.Nil(t, result.state.services["web"], "Old service 'web' should be removed")
}

func Test_HandleProfileResolved_ResetsSelection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := Model{log: newTestLogger(ctrl), loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.tiers = make([]Tier, 0)
	m.state.selected = 5

	event := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "dev",
			Tiers:   []bus.Tier{{Name: "tier1", Services: []string{"db"}}},
		},
	}

	result := m.handleProfileResolved(event)
	assert.Equal(t, 0, result.state.selected, "Selection should reset to 0 on profile reload")
}

func Test_HandleProfileResolved_PreservesReadyState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := Model{log: newTestLogger(ctrl), loader: NewLoader()}
	m.state.services = make(map[string]*ServiceState)
	m.state.tiers = make([]Tier, 0)
	m.state.ready = true

	event := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "dev",
			Tiers:   []bus.Tier{{Name: "tier1", Services: []string{"db"}}},
		},
	}

	result := m.handleProfileResolved(event)
	assert.True(t, result.state.ready, "Ready state should be preserved on profile reload")
}

func Test_HandleProfileResolved_ClearsLoaderQueue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	loader := NewLoader()
	loader.Start("old-service-1", "starting old-service-1")
	loader.Start("old-service-2", "starting old-service-2")

	m := Model{log: newTestLogger(ctrl), loader: loader}
	m.state.services = make(map[string]*ServiceState)
	m.state.tiers = make([]Tier, 0)

	assert.True(t, loader.Active, "Loader should be active before reload")
	assert.True(t, loader.Has("old-service-1"), "Loader should have old-service-1")
	assert.True(t, loader.Has("old-service-2"), "Loader should have old-service-2")

	event := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "new-profile",
			Tiers:   []bus.Tier{{Name: "tier1", Services: []string{"new-service"}}},
		},
	}

	result := m.handleProfileResolved(event)
	assert.False(t, result.loader.Active, "Loader should be cleared after profile reload")
	assert.False(t, result.loader.Has("old-service-1"), "Loader should not have old-service-1")
	assert.False(t, result.loader.Has("old-service-2"), "Loader should not have old-service-2")
}

func Test_HandleTierStarting(t *testing.T) {
	m := Model{}
	m.state.tiers = []Tier{
		{Name: "tier1", Ready: true},
		{Name: "tier2", Ready: true},
	}

	event := bus.Message{Type: bus.EventTierStarting, Data: bus.TierStarting{Name: "tier1"}}
	result := m.handleTierStarting(event)

	assert.False(t, result.state.tiers[0].Ready)
	assert.True(t, result.state.tiers[1].Ready)
}

func Test_HandleTierStarting_InvalidData(t *testing.T) {
	m := Model{}
	m.state.tiers = []Tier{{Name: "tier1", Ready: true}}
	event := bus.Message{Type: bus.EventTierStarting, Data: "invalid"}
	result := m.handleTierStarting(event)
	assert.True(t, result.state.tiers[0].Ready)
}

func Test_HandleTierReady(t *testing.T) {
	m := Model{}
	m.state.tiers = []Tier{
		{Name: "tier1", Ready: false},
		{Name: "tier2", Ready: false},
	}

	event := bus.Message{Type: bus.EventTierReady, Data: bus.Payload{Name: "tier2"}}
	result := m.handleTierReady(event)

	assert.False(t, result.state.tiers[0].Ready)
	assert.True(t, result.state.tiers[1].Ready)
}

func Test_HandleTierReady_InvalidData(t *testing.T) {
	m := Model{}
	m.state.tiers = []Tier{{Name: "tier1", Ready: false}}
	event := bus.Message{Type: bus.EventTierReady, Data: "invalid"}
	result := m.handleTierReady(event)
	assert.False(t, result.state.tiers[0].Ready)
}

func Test_HandleServiceStarting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := bus.NewMockBus(ctrl)
	mockController := NewMockController(ctrl)
	mockLog := newTestLogger(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	service := &ServiceState{Name: "api", Status: StatusStopped}
	m := Model{
		ctx:        context.Background(),
		loader:     loader,
		bus:        mockCmd,
		controller: mockController,
		log:        mockLog,
	}
	m.state.services = map[string]*ServiceState{"api": service}
	service.FSM = newServiceFSM(service, loader, mockLog)

	mockController.EXPECT().HandleStarting(gomock.Any(), service, 1234).Do(
		func(_ context.Context, s *ServiceState, pid int) {
			s.Monitor.PID = pid
		},
	)

	event := bus.Message{
		Timestamp: time.Now(),
		Type:      bus.EventServiceStarting,
		Data:      bus.ServiceStarting{ServiceEvent: bus.ServiceEvent{Service: "api", Tier: "tier1"}, PID: 1234},
	}

	result := m.handleServiceStarting(event)

	assert.Equal(t, "tier1", result.state.services["api"].Tier)
	assert.Equal(t, 1234, result.state.services["api"].Monitor.PID)
}

func Test_HandleServiceStarting_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.services = make(map[string]*ServiceState)
	event := bus.Message{Type: bus.EventServiceStarting, Data: "invalid"}
	result := m.handleServiceStarting(event)
	assert.False(t, result.loader.Active)
}

func Test_HandleServiceReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("api", "starting api…")

	service := &ServiceState{Name: "api", Status: StatusStarting}
	m := Model{
		ctx:        context.Background(),
		loader:     loader,
		controller: mockController,
	}
	m.state.services = map[string]*ServiceState{"api": service}

	mockController.EXPECT().HandleReady(gomock.Any(), service)

	event := bus.Message{
		Timestamp: time.Now(),
		Type:      bus.EventServiceReady,
		Data:      bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "api"}},
	}

	result := m.handleServiceReady(event)

	assert.False(t, result.loader.Active)
	assert.NotZero(t, result.state.services["api"].Monitor.ReadyTime)
}

func Test_HandleServiceReady_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("api", "starting api…")
	m := Model{loader: loader}
	m.state.services = make(map[string]*ServiceState)
	event := bus.Message{Type: bus.EventServiceReady, Data: "invalid"}
	result := m.handleServiceReady(event)
	assert.True(t, result.loader.Active)
}

func Test_HandleServiceFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("api", "starting api…")

	service := &ServiceState{Name: "api", Status: StatusStarting}
	m := Model{
		ctx:        context.Background(),
		loader:     loader,
		controller: mockController,
	}
	m.state.services = map[string]*ServiceState{"api": service}

	mockController.EXPECT().HandleFailed(gomock.Any(), service)

	testErr := assert.AnError
	event := bus.Message{
		Type: bus.EventServiceFailed,
		Data: bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: "api"}, Error: testErr},
	}

	result := m.handleServiceFailed(event)

	assert.False(t, result.loader.Active)
	assert.Equal(t, testErr, result.state.services["api"].Error)
}

func Test_HandleServiceFailed_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.services = make(map[string]*ServiceState)
	event := bus.Message{Type: bus.EventServiceFailed, Data: "invalid"}
	result := m.handleServiceFailed(event)
	assert.Len(t, result.state.services, 0)
}

func Test_HandleServiceStopped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("api", "stopping api…")

	service := &ServiceState{Name: "api", Status: StatusRunning, Monitor: ServiceMonitor{PID: 1234}}
	m := Model{
		ctx:        context.Background(),
		loader:     loader,
		controller: mockController,
	}
	m.state.services = map[string]*ServiceState{"api": service}

	mockController.EXPECT().HandleStopped(gomock.Any(), service).Return(false)

	event := bus.Message{Type: bus.EventServiceStopped, Data: bus.ServiceStopped{ServiceEvent: bus.ServiceEvent{Service: "api"}}}
	result := m.handleServiceStopped(event)

	assert.False(t, result.loader.Active)
}

func Test_HandleServiceStopped_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.services = make(map[string]*ServiceState)
	event := bus.Message{Type: bus.EventServiceStopped, Data: "invalid"}
	result := m.handleServiceStopped(event)
	assert.Len(t, result.state.services, 0)
}

func Test_HandleWatchStarted(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusRunning, Watching: false}
	m := Model{}
	m.state.services = map[string]*ServiceState{"api": service}

	event := bus.Message{
		Type: bus.EventWatchStarted,
		Data: bus.Payload{Name: "api"},
	}

	result := m.handleWatchStarted(event)

	assert.True(t, result.state.services["api"].Watching)
}

func Test_HandleWatchStarted_InvalidData(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusRunning, Watching: false}
	m := Model{}
	m.state.services = map[string]*ServiceState{"api": service}

	event := bus.Message{Type: bus.EventWatchStarted, Data: "invalid"}
	result := m.handleWatchStarted(event)

	assert.False(t, result.state.services["api"].Watching)
}

func Test_HandleWatchStarted_UnknownService(t *testing.T) {
	m := Model{}
	m.state.services = make(map[string]*ServiceState)

	event := bus.Message{
		Type: bus.EventWatchStarted,
		Data: bus.Payload{Name: "unknown"},
	}

	result := m.handleWatchStarted(event)

	assert.Len(t, result.state.services, 0)
}

func Test_HandleWatchStopped(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusStopped, Watching: true}
	m := Model{}
	m.state.services = map[string]*ServiceState{"api": service}

	event := bus.Message{
		Type: bus.EventWatchStopped,
		Data: bus.Payload{Name: "api"},
	}

	result := m.handleWatchStopped(event)

	assert.False(t, result.state.services["api"].Watching)
}

func Test_HandleWatchStopped_InvalidData(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusStopped, Watching: true}
	m := Model{}
	m.state.services = map[string]*ServiceState{"api": service}

	event := bus.Message{Type: bus.EventWatchStopped, Data: "invalid"}
	result := m.handleWatchStopped(event)

	assert.True(t, result.state.services["api"].Watching)
}

func Test_HandleWatchStopped_UnknownService(t *testing.T) {
	m := Model{}
	m.state.services = make(map[string]*ServiceState)

	event := bus.Message{
		Type: bus.EventWatchStopped,
		Data: bus.Payload{Name: "unknown"},
	}

	result := m.handleWatchStopped(event)

	assert.Len(t, result.state.services, 0)
}

func Test_HandlePhaseChanged_PhaseStopped(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("_shutdown", "shutting down…")

	m := Model{loader: loader}
	m.state.shuttingDown = true

	event := bus.Message{Type: bus.EventPhaseChanged, Data: bus.PhaseChanged{Phase: bus.PhaseStopped}}
	result, cmd := m.handlePhaseChanged(event)

	assert.False(t, result.loader.Active)
	assert.NotNil(t, cmd)
}

func Test_HandlePhaseChanged_OtherPhase(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	msgChan := make(chan bus.Message, 1)

	m := Model{loader: loader, msgChan: msgChan}

	event := bus.Message{Type: bus.EventPhaseChanged, Data: bus.PhaseChanged{Phase: bus.PhaseRunning}}
	result, cmd := m.handlePhaseChanged(event)

	assert.Equal(t, bus.PhaseRunning, result.state.phase)
	assert.NotNil(t, cmd)
}

func Test_HandleEvent_SignalCaught(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	msgChan := make(chan bus.Message, 1)

	m := Model{loader: loader, msgChan: msgChan}
	m.state.shuttingDown = false

	event := bus.Message{Type: bus.EventSignal, Data: bus.Signal{Name: "SIGINT"}}
	teaModel, cmd := m.handleMessage(event)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.True(t, result.loader.Active)
	assert.Equal(t, "shutting down all services…", result.loader.Message())
	assert.NotNil(t, cmd)
}

func Test_HandleKeyPress_ForceQuitWithCtrlC(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil)

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController, log: mockLogger}
	m.state.shuttingDown = false
	m.ui.servicesKeys = DefaultKeyMap()

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	teaModel, cmd := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.False(t, result.loader.Active)
	assert.NotNil(t, cmd)
}

func Test_HandleKeyPress_IgnoresQuitWhileShuttingDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.shuttingDown = true
	m.ui.servicesKeys = DefaultKeyMap()

	msg := toKeyMsg("q")
	teaModel, cmd := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.Nil(t, cmd)
}

func Test_HandleKeyPress_IgnoresOtherKeysWhileShuttingDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.shuttingDown = true
	m.ui.servicesKeys = DefaultKeyMap()

	msg := toKeyMsg("j")
	teaModel, cmd := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.Nil(t, cmd)
}

func Test_HandleKeyPress_QuitStartsGracefulShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	mockController.EXPECT().StopAll()

	msgChan := make(chan bus.Message, 1)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController, msgChan: msgChan}
	m.state.shuttingDown = false
	m.ui.servicesKeys = DefaultKeyMap()

	msg := toKeyMsg("q")
	teaModel, cmd := m.handleKeyPress(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.True(t, result.loader.Active)
	assert.Equal(t, "shutting down all services…", result.loader.Message())
	assert.NotNil(t, cmd)
}

func Test_HandleKeyPress_CtrlCForcesImmediateQuit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil)

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController, log: mockLogger}
	m.state.shuttingDown = false
	m.ui.servicesKeys = DefaultKeyMap()

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.handleKeyPress(msg)

	assert.NotNil(t, cmd)
}

func Test_Update_KeyMsg_CtrlCForceQuitsDuringShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil)

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController, log: mockLogger}
	m.state.shuttingDown = true
	m.ui.servicesKeys = DefaultKeyMap()

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)

	assert.NotNil(t, cmd)
}

func Test_Update_KeyMsg_IgnoreQuitDuringShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockController := NewMockController(ctrl)
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, controller: mockController}
	m.state.shuttingDown = true
	m.ui.servicesKeys = DefaultKeyMap()

	msg := toKeyMsg("q")
	teaModel, cmd := m.Update(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.Nil(t, cmd)
}

func Test_Update_EventMsg(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	msgChan := make(chan bus.Message, 1)

	m := Model{loader: loader, msgChan: msgChan}
	m.state.shuttingDown = false

	event := bus.Message{Type: bus.EventSignal, Data: bus.Signal{Name: "SIGINT"}}
	msg := msgMsg(event)
	teaModel, cmd := m.Update(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.NotNil(t, cmd)
}

func toKeyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
