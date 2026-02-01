package services

import (
	"testing"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/app/ui/components"
	"fuku/internal/config/logger"
)

func setupViewTestLogger(ctrl *gomock.Controller) logger.Logger {
	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	return mockLogger
}

func Test_View_NotReady(t *testing.T) {
	m := Model{}
	m.state.ready = false

	result := m.View()
	assert.Equal(t, "Initializing…", result)
}

func Test_View_RendersWhileShuttingDown(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: true, queue: []LoaderItem{{Service: "_shutdown", Message: "Shutting down…"}}}
	m := Model{loader: loader}
	m.state.ready = true
	m.state.shuttingDown = true
	m.state.phase = bus.PhaseStopping
	m.state.services = map[string]*ServiceState{"api": {Name: "api", Status: StatusRunning}}
	m.state.tiers = []Tier{{Name: "tier1", Services: []string{"api"}}}
	m.ui.width = 100
	m.ui.height = 50
	m.ui.help = help.New()
	m.ui.servicesKeys = DefaultKeyMap()
	m.ui.servicesViewport = viewport.New(80, 30)

	result := m.View()

	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Shutting down")
}

func Test_RenderTitle_ServicesView(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: false, queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.phase = bus.PhaseRunning
	m.state.services = map[string]*ServiceState{"api": {Status: StatusRunning}}
	m.state.tiers = []Tier{{Services: []string{"api"}}}
	m.ui.width = 100

	title := m.renderTitle()
	assert.Contains(t, title, "services")
}

func Test_RenderTitle_WithActiveLoader(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: true, queue: make([]LoaderItem, 0)}
	loader.Start("api", "Starting api…")
	m := Model{loader: loader}
	m.state.phase = bus.PhaseStartup
	m.state.services = make(map[string]*ServiceState)
	m.state.tiers = []Tier{}
	m.ui.width = 100

	title := m.renderTitle()
	assert.Contains(t, title, "Starting api…")
}

func Test_RenderStatus_PhaseColors(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: false, queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}

	m.state.services = make(map[string]*ServiceState)
	m.state.tiers = []Tier{}

	tests := []struct {
		name         string
		phase        bus.Phase
		wantContains string
	}{
		{name: "startup phase", phase: bus.PhaseStartup, wantContains: "Starting…"},
		{name: "running phase", phase: bus.PhaseRunning, wantContains: "Running"},
		{name: "stopping phase", phase: bus.PhaseStopping, wantContains: "Stopping"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.state.phase = tt.phase

			info := m.renderStatus()
			assert.Contains(t, info, tt.wantContains)
		})
	}
}

func Test_RenderServices_Empty(t *testing.T) {
	m := Model{}
	m.state.tiers = []Tier{}
	m.ui.servicesViewport = viewport.New(80, 20)

	result := m.renderServices()
	assert.Contains(t, result, "No services configured")
}

func Test_GetStyledAndPaddedStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     Status
		isSelected bool
	}{
		{
			name:       "running status not selected",
			status:     StatusRunning,
			isSelected: false,
		},
		{
			name:       "starting status not selected",
			status:     StatusStarting,
			isSelected: false,
		},
		{
			name:       "failed status not selected",
			status:     StatusFailed,
			isSelected: false,
		},
		{
			name:       "stopped status not selected",
			status:     StatusStopped,
			isSelected: false,
		},
		{
			name:       "running status selected",
			status:     StatusRunning,
			isSelected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			service := &ServiceState{Status: tt.status}

			result := m.getStyledAndPaddedStatus(service, tt.isSelected)
			assert.Contains(t, result, string(tt.status))
		})
	}
}

func Test_GetStyledAndPaddedStatus_NoWatchIndicator(t *testing.T) {
	tests := []struct {
		name       string
		status     Status
		watching   bool
		isSelected bool
	}{
		{
			name:       "running with watching - indicator in indicator column not status",
			status:     StatusRunning,
			watching:   true,
			isSelected: false,
		},
		{
			name:       "running without watching",
			status:     StatusRunning,
			watching:   false,
			isSelected: false,
		},
		{
			name:       "running with watching selected",
			status:     StatusRunning,
			watching:   true,
			isSelected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			service := &ServiceState{Status: tt.status, Watching: tt.watching}

			result := m.getStyledAndPaddedStatus(service, tt.isSelected)
			assert.Contains(t, result, string(tt.status))
			assert.NotContains(t, result, components.IndicatorWatch)
		})
	}
}

func Test_RenderServiceRow_Truncation(t *testing.T) {
	tests := []struct {
		name          string
		serviceName   string
		maxNameLen    int
		viewportWidth int
		wantTruncated bool
		wantNameInRow string
	}{
		{
			name:          "short name no truncation",
			serviceName:   "api",
			maxNameLen:    20,
			viewportWidth: 100,
			wantTruncated: false,
			wantNameInRow: "api",
		},
		{
			name:          "long name truncated",
			serviceName:   "action-confirmation-management-service",
			maxNameLen:    38,
			viewportWidth: 60,
			wantTruncated: true,
			wantNameInRow: "action-confirmation…",
		},
		{
			name:          "name fits exactly",
			serviceName:   "user-service",
			maxNameLen:    12,
			viewportWidth: 100,
			wantTruncated: false,
			wantNameInRow: "user-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.ui.width = tt.viewportWidth + 8
			m.ui.servicesViewport.Width = tt.viewportWidth
			service := &ServiceState{Name: tt.serviceName, Status: StatusRunning}

			result := m.renderServiceRow(service, false, tt.maxNameLen)

			assert.Contains(t, result, tt.wantNameInRow)

			if tt.wantTruncated {
				assert.Contains(t, result, "…")
			}
		})
	}
}

func Test_RenderServiceRow_ColumnAlignment(t *testing.T) {
	m := Model{}
	m.ui.width = 120
	m.ui.servicesViewport.Width = 112

	service1 := &ServiceState{Name: "api", Status: StatusRunning}
	service2 := &ServiceState{Name: "user-management-service", Status: StatusStarting}

	row1 := m.renderServiceRow(service1, false, 25)
	row2 := m.renderServiceRow(service2, false, 25)

	assert.Contains(t, row1, "api")
	assert.Contains(t, row1, "Running")
	assert.Contains(t, row2, "user-management-service")
	assert.Contains(t, row2, "Starting")
}

func Test_RenderServiceRow_SelectedIndicator(t *testing.T) {
	m := Model{}
	m.ui.width = 100
	m.ui.servicesViewport.Width = 92

	service := &ServiceState{Name: "api", Status: StatusRunning}

	notSelected := m.renderServiceRow(service, false, 20)
	selected := m.renderServiceRow(service, true, 20)

	assert.Contains(t, notSelected, "  ")
	assert.Contains(t, selected, components.IndicatorSelected+" ")
}

func Test_GetServiceDetails_WithError(t *testing.T) {
	m := Model{}

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "port already in use",
			err:      errors.ErrPortAlreadyInUse,
			expected: "port already in use",
		},
		{
			name:     "max retries exceeded",
			err:      errors.ErrMaxRetriesExceeded,
			expected: "max retries exceeded",
		},
		{
			name:     "readiness timeout",
			err:      errors.ErrReadinessTimeout,
			expected: "readiness timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &ServiceState{
				Name:   "api",
				Status: StatusFailed,
				Error:  tt.err,
			}

			result := m.getServiceDetails(service, true)

			assert.Contains(t, result, tt.expected)
		})
	}
}

func Test_GetServiceDetails_WithMetrics(t *testing.T) {
	m := Model{}

	service := &ServiceState{
		Name:   "api",
		Status: StatusRunning,
		Monitor: ServiceMonitor{
			PID: 12345,
			CPU: 5.5,
			MEM: 128,
		},
	}

	result := m.getServiceDetails(service, false)

	assert.Contains(t, result, "5.5%")
	assert.Contains(t, result, "128MB")
	assert.Contains(t, result, "12345")
}

func Test_GetServiceDetails_NoMetricsWhenStopped(t *testing.T) {
	m := Model{}

	service := &ServiceState{
		Name:   "api",
		Status: StatusStopped,
	}

	result := m.getServiceDetails(service, false)

	assert.NotContains(t, result, "%")
	assert.NotContains(t, result, "MB")
}

func Test_RenderTier_Spacing(t *testing.T) {
	m := Model{}
	m.state.services = map[string]*ServiceState{
		"api": {Name: "api", Status: StatusRunning},
		"db":  {Name: "db", Status: StatusRunning},
	}
	m.ui.width = 100
	m.ui.servicesViewport.Width = 92

	tier := Tier{Name: "platform", Services: []string{"api", "db"}}
	currentIdx := 0

	result := m.renderTier(tier, &currentIdx, 20)

	assert.Contains(t, result, "platform")
	assert.Contains(t, result, "api")
	assert.Contains(t, result, "db")
}

func Test_RenderTier_ServiceCount(t *testing.T) {
	m := Model{}
	m.state.services = map[string]*ServiceState{
		"api": {Name: "api", Status: StatusRunning},
		"db":  {Name: "db", Status: StatusRunning},
		"web": {Name: "web", Status: StatusRunning},
	}
	m.ui.width = 100
	m.ui.servicesViewport.Width = 92

	tier := Tier{Name: "platform", Services: []string{"api", "db", "web"}}
	currentIdx := 0

	result := m.renderTier(tier, &currentIdx, 20)

	assert.Equal(t, 3, currentIdx)
	assert.Contains(t, result, "api")
	assert.Contains(t, result, "db")
	assert.Contains(t, result, "web")
}

func Test_GetServiceIndicator_DefaultNotSelected(t *testing.T) {
	m := Model{}
	service := &ServiceState{Name: "api", Status: StatusStopped}

	result := m.getServiceIndicator(service, false)
	assert.Equal(t, " ", result)
}

func Test_GetServiceIndicator_DefaultSelected(t *testing.T) {
	m := Model{}
	service := &ServiceState{Name: "api", Status: StatusStopped}

	result := m.getServiceIndicator(service, true)
	assert.Equal(t, components.IndicatorSelected, result)
}

func Test_GetServiceIndicator_GuardFSMNil(t *testing.T) {
	m := Model{}

	tests := []struct {
		name       string
		status     Status
		watching   bool
		isSelected bool
		want       string
	}{
		{
			name:       "FSM nil not selected",
			status:     StatusRunning,
			watching:   false,
			isSelected: false,
			want:       " ",
		},
		{
			name:       "FSM nil selected",
			status:     StatusRunning,
			watching:   false,
			isSelected: true,
			want:       components.IndicatorSelected,
		},
		{
			name:       "FSM nil watching running not selected shows watch indicator",
			status:     StatusRunning,
			watching:   true,
			isSelected: false,
			want:       components.IndicatorWatchStyle.Render(components.IndicatorWatch),
		},
		{
			name:       "FSM nil watching running selected shows watch indicator unstyled",
			status:     StatusRunning,
			watching:   true,
			isSelected: true,
			want:       components.IndicatorWatch,
		},
		{
			name:       "FSM nil watching stopped does not show watch indicator",
			status:     StatusStopped,
			watching:   true,
			isSelected: false,
			want:       " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &ServiceState{Name: "api", Status: tt.status, Watching: tt.watching, FSM: nil}

			result := m.getServiceIndicator(service, tt.isSelected)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_GetServiceIndicator_GuardNonTransitionalState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := Model{}
	service := &ServiceState{Name: "api", Status: StatusRunning}
	service.FSM = newServiceFSM(service, newTestLoader(), setupViewTestLogger(ctrl))

	tests := []struct {
		name       string
		state      string
		isSelected bool
		want       string
	}{
		{
			name:       "Running state not selected",
			state:      Running,
			isSelected: false,
			want:       " ",
		},
		{
			name:       "Running state selected",
			state:      Running,
			isSelected: true,
			want:       components.IndicatorSelected,
		},
		{
			name:       "Stopped state not selected",
			state:      Stopped,
			isSelected: false,
			want:       " ",
		},
		{
			name:       "Failed state not selected",
			state:      Failed,
			isSelected: false,
			want:       " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service.FSM.SetState(tt.state)

			result := m.getServiceIndicator(service, tt.isSelected)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_GetServiceIndicator_WatchingWithFSM(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := Model{}
	service := &ServiceState{Name: "api", Status: StatusRunning, Watching: true}
	service.FSM = newServiceFSM(service, newTestLoader(), setupViewTestLogger(ctrl))

	tests := []struct {
		name       string
		state      string
		isSelected bool
		want       string
	}{
		{
			name:       "Running state watching not selected shows watch indicator",
			state:      Running,
			isSelected: false,
			want:       components.IndicatorWatchStyle.Render(components.IndicatorWatch),
		},
		{
			name:       "Running state watching selected shows unstyled watch indicator",
			state:      Running,
			isSelected: true,
			want:       components.IndicatorWatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service.FSM.SetState(tt.state)

			result := m.getServiceIndicator(service, tt.isSelected)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_GetWatchIndicator(t *testing.T) {
	m := Model{}

	tests := []struct {
		name       string
		isSelected bool
		want       string
	}{
		{
			name:       "not selected returns styled indicator",
			isSelected: false,
			want:       components.IndicatorWatchStyle.Render(components.IndicatorWatch),
		},
		{
			name:       "selected returns unstyled indicator",
			isSelected: true,
			want:       components.IndicatorWatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.getWatchIndicator(tt.isSelected)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_GetServiceIndicator_GuardBlinkNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := Model{}
	service := &ServiceState{Name: "api", Status: StatusRunning, Blink: nil}
	service.FSM = newServiceFSM(service, newTestLoader(), setupViewTestLogger(ctrl))

	tests := []struct {
		name       string
		state      string
		isSelected bool
		want       string
	}{
		{
			name:       "Starting state Blink nil not selected",
			state:      Starting,
			isSelected: false,
			want:       " ",
		},
		{
			name:       "Stopping state Blink nil selected",
			state:      Stopping,
			isSelected: true,
			want:       components.IndicatorSelected,
		},
		{
			name:       "Restarting state Blink nil not selected",
			state:      Restarting,
			isSelected: false,
			want:       " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service.FSM.SetState(tt.state)

			result := m.getServiceIndicator(service, tt.isSelected)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_GetServiceIndicator_BlinkIndicatorNotSelected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := Model{}
	blink := components.NewBlink()
	service := &ServiceState{Name: "api", Status: StatusRunning, Blink: blink}
	service.FSM = newServiceFSM(service, newTestLoader(), setupViewTestLogger(ctrl))

	tests := []struct {
		name  string
		state string
	}{
		{
			name:  "Starting state with blink",
			state: Starting,
		},
		{
			name:  "Stopping state with blink",
			state: Stopping,
		},
		{
			name:  "Restarting state with blink",
			state: Restarting,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service.FSM.SetState(tt.state)

			result := m.getServiceIndicator(service, false)
			assert.NotEqual(t, " ", result)
			assert.NotEmpty(t, result)
		})
	}
}

func Test_GetServiceIndicator_BlinkIndicatorSelected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := Model{}
	blink := components.NewBlink()
	service := &ServiceState{Name: "api", Status: StatusRunning, Blink: blink}
	service.FSM = newServiceFSM(service, newTestLoader(), setupViewTestLogger(ctrl))

	tests := []struct {
		name  string
		state string
	}{
		{
			name:  "Starting state selected",
			state: Starting,
		},
		{
			name:  "Stopping state selected",
			state: Stopping,
		},
		{
			name:  "Restarting state selected",
			state: Restarting,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service.FSM.SetState(tt.state)

			result := m.getServiceIndicator(service, true)
			assert.Equal(t, blink.Frame(), result)
		})
	}
}

func Test_RenderTip_RotatesOverTime(t *testing.T) {
	tipCount := len(components.Tips)
	wrapTick := tipCount * components.TipRotationTicks

	tests := []struct {
		name        string
		tipOffset   int
		tickCounter int
		wantIndex   int
	}{
		{
			name:        "tick 0 with offset 0",
			tipOffset:   0,
			tickCounter: 0,
			wantIndex:   0,
		},
		{
			name:        "tick 100 with offset 0",
			tipOffset:   0,
			tickCounter: 100,
			wantIndex:   1,
		},
		{
			name:        "tick 0 with offset 3",
			tipOffset:   3,
			tickCounter: 0,
			wantIndex:   3,
		},
		{
			name:        "wraps after all tips",
			tipOffset:   0,
			tickCounter: wrapTick,
			wantIndex:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.ui.tipOffset = tt.tipOffset
			m.ui.tickCounter = tt.tickCounter
			m.ui.showTips = true

			result := m.renderTip()
			assert.Equal(t, components.Tips[tt.wantIndex], result)
		})
	}
}

func Test_RenderTip_HiddenWhenDisabled(t *testing.T) {
	m := Model{}
	m.ui.showTips = false
	m.ui.tickCounter = 0

	result := m.renderTip()
	assert.Empty(t, result)
}
