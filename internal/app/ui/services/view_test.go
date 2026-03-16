package services

import (
	"testing"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/app/ui/components"
	"fuku/internal/config/logger"
)

func layoutForWidth(rowWidth int) components.TableLayout {
	return components.ComputeTableLayout(rowWidth - components.RowHorizontalPadding)
}

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
	assert.Equal(t, tea.NewView("initializing…"), result)
}

func Test_View_RendersWhileShuttingDown(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: true, queue: []LoaderItem{{Service: loaderKeyShutdown, Message: "shutting down…"}}}
	m := Model{loader: loader}
	m.state.ready = true
	m.state.shuttingDown = true
	m.state.phase = bus.PhaseStopping
	m.state.services = map[string]*ServiceState{"api": {Name: "api", Status: StatusRunning}}
	m.state.tiers = []Tier{{Name: "tier1", Services: []string{"api"}}}
	m.ui.width = 100
	m.ui.height = 50
	m.ui.layout = layoutForWidth(100 - components.PanelInnerPadding)
	m.ui.help = help.New()
	m.ui.servicesKeys = DefaultKeyMap()
	m.ui.servicesViewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(30))
	m.theme = components.DefaultTheme()

	result := m.View()

	assert.NotEmpty(t, result.Content)
	assert.Contains(t, result.Content, "shutting down")
	assert.True(t, result.AltScreen)
}

func Test_RenderTitle_ServicesView(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: false, queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.profile = "default"
	m.state.phase = bus.PhaseRunning
	m.state.services = map[string]*ServiceState{"api": {Status: StatusRunning}}
	m.state.tiers = []Tier{{Services: []string{"api"}}}
	m.ui.width = 100

	title := m.renderTitle()
	assert.Equal(t, "profile • default", title)
}

func Test_RenderTitle_WithActiveLoader(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: true, queue: make([]LoaderItem, 0)}
	loader.Start("api", "starting api…")
	m := Model{loader: loader}
	m.state.phase = bus.PhaseStartup
	m.state.services = make(map[string]*ServiceState)
	m.state.tiers = []Tier{}
	m.ui.width = 100

	title := m.renderTitle()
	assert.Contains(t, title, "starting api…")
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
		{name: "startup phase", phase: bus.PhaseStartup, wantContains: "starting…"},
		{name: "running phase", phase: bus.PhaseRunning, wantContains: "running"},
		{name: "stopping phase", phase: bus.PhaseStopping, wantContains: "stopping"},
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
	m.ui.servicesViewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))

	result := m.renderServices()
	assert.Contains(t, result, "no services configured")
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
			m.ui.layout = layoutForWidth(78)
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
			m.ui.layout = layoutForWidth(78)
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
		viewportWidth int
		wantTruncated bool
		wantNameInRow string
	}{
		{
			name:          "short name no truncation",
			serviceName:   "api",
			viewportWidth: 100,
			wantTruncated: false,
			wantNameInRow: "api",
		},
		{
			name:          "long name truncated on narrow viewport",
			serviceName:   "action-confirmation-management-service",
			viewportWidth: 78,
			wantTruncated: true,
			wantNameInRow: "action-confirmation-managemen…",
		},
		{
			name:          "name fits exactly",
			serviceName:   "user-service",
			viewportWidth: 100,
			wantTruncated: false,
			wantNameInRow: "user-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.ui.width = tt.viewportWidth + 8
			m.ui.layout = layoutForWidth(tt.viewportWidth)
			m.ui.servicesViewport.SetWidth(tt.viewportWidth)
			service := &ServiceState{Name: tt.serviceName, Status: StatusRunning}

			result := m.renderServiceRow(service, false)

			assert.Contains(t, result, tt.wantNameInRow)

			if tt.wantTruncated {
				assert.Contains(t, result, "…")
			}
		})
	}
}

func Test_RenderNoWrapAtBreakpoints(t *testing.T) {
	tests := []struct {
		name        string
		panelWidth  int
		serviceName string
	}{
		{
			name:        "72-col terminal",
			panelWidth:  72,
			serviceName: "test-service",
		},
		{
			name:        "104-col terminal",
			panelWidth:  104,
			serviceName: "long-service-name-xx",
		},
		{
			name:        "120-col terminal",
			panelWidth:  120,
			serviceName: "long-service-name-xx",
		},
		{
			name:        "200-col terminal",
			panelWidth:  200,
			serviceName: "long-service-name-xx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rowWidth := tt.panelWidth - components.PanelInnerPadding

			m := Model{}
			m.ui.width = tt.panelWidth
			m.ui.layout = layoutForWidth(rowWidth)
			m.ui.servicesViewport.SetWidth(rowWidth)
			m.theme = components.DefaultTheme()

			service := &ServiceState{
				Name:   tt.serviceName,
				Status: StatusRunning,
				Monitor: ServiceMonitor{
					PID: 12345,
					CPU: 99.9,
					MEM: 512,
				},
			}

			header := m.renderColumnHeaders()
			row := m.renderServiceRow(service, false)

			assert.NotContains(t, header, "\n", "header wraps")
			assert.NotContains(t, row, "\n", "service row wraps")
			assert.Equal(t, rowWidth, lipgloss.Width(header), "header width must equal rowWidth")
			assert.Equal(t, rowWidth, lipgloss.Width(row), "service row width must equal rowWidth")
		})
	}
}

func Test_RenderServiceRow_ColumnAlignment(t *testing.T) {
	m := Model{}
	m.ui.width = 120
	m.ui.layout = layoutForWidth(112)
	m.ui.servicesViewport.SetWidth(112)

	service1 := &ServiceState{Name: "api", Status: StatusRunning}
	service2 := &ServiceState{Name: "user-management-service", Status: StatusStarting}

	row1 := m.renderServiceRow(service1, false)
	row2 := m.renderServiceRow(service2, false)

	assert.Contains(t, row1, "api")
	assert.Contains(t, row1, "running")
	assert.Contains(t, row2, "user-management-service")
	assert.Contains(t, row2, "starting")
}

func Test_RenderServiceRow_SelectedIndicator(t *testing.T) {
	m := Model{}
	m.ui.width = 100
	m.ui.layout = layoutForWidth(92)
	m.ui.servicesViewport.SetWidth(92)

	service := &ServiceState{Name: "api", Status: StatusRunning}

	notSelected := m.renderServiceRow(service, false)
	selected := m.renderServiceRow(service, true)

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
	m.ui.layout = layoutForWidth(78)

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
	m.ui.layout = layoutForWidth(78)

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
	m.ui.layout = layoutForWidth(92)
	m.ui.servicesViewport.SetWidth(92)

	tier := Tier{Name: "platform", Services: []string{"api", "db"}}
	currentIdx := 0

	result := m.renderTier(tier, &currentIdx)

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
	m.ui.layout = layoutForWidth(92)
	m.ui.servicesViewport.SetWidth(92)

	tier := Tier{Name: "platform", Services: []string{"api", "db", "web"}}
	currentIdx := 0

	result := m.renderTier(tier, &currentIdx)

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
	theme := components.DefaultTheme()
	m := Model{}
	m.theme = theme

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
			want:       theme.IndicatorWatchStyle.Render(components.IndicatorWatch),
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
	service.FSM = newServiceFSM(service, &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, setupViewTestLogger(ctrl))

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

	theme := components.DefaultTheme()
	m := Model{}
	m.theme = theme
	service := &ServiceState{Name: "api", Status: StatusRunning, Watching: true}
	service.FSM = newServiceFSM(service, &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, setupViewTestLogger(ctrl))

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
			want:       theme.IndicatorWatchStyle.Render(components.IndicatorWatch),
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
	theme := components.DefaultTheme()
	m := Model{}
	m.theme = theme

	tests := []struct {
		name       string
		isSelected bool
		want       string
	}{
		{
			name:       "not selected returns styled indicator",
			isSelected: false,
			want:       theme.IndicatorWatchStyle.Render(components.IndicatorWatch),
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
	service.FSM = newServiceFSM(service, &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, setupViewTestLogger(ctrl))

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
	service.FSM = newServiceFSM(service, &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, setupViewTestLogger(ctrl))

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
	service.FSM = newServiceFSM(service, &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, setupViewTestLogger(ctrl))

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
			theme := components.DefaultTheme()
			m := Model{}
			m.theme = theme
			m.ui.tipOffset = tt.tipOffset
			m.ui.tickCounter = tt.tickCounter
			m.ui.showTips = true

			result := m.renderTip()
			assert.Equal(t, components.Tips[tt.wantIndex].Render(theme), result)
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
