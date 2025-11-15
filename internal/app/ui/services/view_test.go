package services

import (
	"testing"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/stretchr/testify/assert"

	"fuku/internal/app/runtime"
)

func Test_View_NotReady(t *testing.T) {
	m := Model{ready: false}
	result := m.View()
	assert.Equal(t, "Initializing…", result)
}

func Test_View_Quitting(t *testing.T) {
	m := Model{ready: true, quitting: true}
	result := m.View()
	assert.Equal(t, "", result)
}

func Test_RenderTitle_ServicesView(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: false, queue: make([]LoaderItem, 0)}
	m := Model{
		viewMode: ViewModeServices,
		phase:    runtime.PhaseRunning,
		services: map[string]*ServiceState{"api": {Status: StatusReady}},
		tiers:    []TierView{{Services: []string{"api"}}},
		loader:   loader,
	}

	title := m.renderTitle()
	assert.Contains(t, title, ">_ services")
	assert.Contains(t, title, "Running")
	assert.Contains(t, title, "1/1")
}

func Test_RenderTitle_LogsView(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: false, queue: make([]LoaderItem, 0)}
	m := Model{
		viewMode: ViewModeLogs,
		phase:    runtime.PhaseRunning,
		services: make(map[string]*ServiceState),
		tiers:    []TierView{},
		loader:   loader,
	}

	title := m.renderTitle()
	assert.Contains(t, title, ">_ logs")
}

func Test_RenderTitle_WithActiveLoader(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: true, queue: make([]LoaderItem, 0)}
	loader.Start("api", "Starting api…")
	m := Model{
		viewMode: ViewModeServices,
		phase:    runtime.PhaseStartup,
		services: make(map[string]*ServiceState),
		tiers:    []TierView{},
		loader:   loader,
	}

	title := m.renderTitle()
	assert.Contains(t, title, "Starting api…")
}

func Test_RenderTitle_PhaseColors(t *testing.T) {
	tests := []struct {
		name         string
		phase        runtime.Phase
		wantContains string
	}{
		{name: "startup phase", phase: runtime.PhaseStartup, wantContains: "Starting…"},
		{name: "running phase", phase: runtime.PhaseRunning, wantContains: "Running"},
		{name: "stopping phase", phase: runtime.PhaseStopping, wantContains: "Stopping"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &Loader{Model: spinner.New(), Active: false, queue: make([]LoaderItem, 0)}
			m := Model{phase: tt.phase, services: make(map[string]*ServiceState), tiers: []TierView{}, loader: loader}
			title := m.renderTitle()
			assert.Contains(t, title, tt.wantContains)
		})
	}
}

func Test_RenderServices_Empty(t *testing.T) {
	m := Model{tiers: []TierView{}, servicesViewport: viewport.New(80, 20)}
	result := m.renderServices()
	assert.Contains(t, result, "No services configured")
}

func Test_RenderLogs_Empty(t *testing.T) {
	vp := viewport.New(80, 20)
	vp.SetContent("")
	m := Model{logsViewport: vp}
	result := m.renderLogs()
	assert.Contains(t, result, "No logs enabled")
}

func Test_RenderHelp(t *testing.T) {
	m := Model{keys: DefaultKeyMap(), help: help.New()}
	result := m.renderHelp()
	assert.NotEmpty(t, result)
}

func Test_ApplyRowStyles_StatusColors(t *testing.T) {
	tests := []struct {
		name   string
		status Status
	}{
		{name: "ready status", status: StatusReady},
		{name: "starting status", status: StatusStarting},
		{name: "failed status", status: StatusFailed},
		{name: "stopped status", status: StatusStopped},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			service := &ServiceState{Status: tt.status}
			row := "  api   " + string(tt.status) + "  25.5%  256MB  00:30"
			result := m.applyRowStyles(row, service)
			assert.Contains(t, result, string(tt.status))
		})
	}
}

func Test_ApplyRowStyles_WithError(t *testing.T) {
	m := Model{}
	service := &ServiceState{Status: StatusFailed, Error: assert.AnError}
	row := "  api   Failed  (assert.AnError general error for testing)"
	result := m.applyRowStyles(row, service)
	assert.Contains(t, result, "Failed")
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
		{name: "short name no truncation", serviceName: "api", maxNameLen: 20, viewportWidth: 100, wantTruncated: false, wantNameInRow: "api"},
		{name: "long name truncated", serviceName: "action-confirmation-management-service", maxNameLen: 38, viewportWidth: 60, wantTruncated: true, wantNameInRow: "action-confirmation…"},
		{name: "name fits exactly", serviceName: "user-service", maxNameLen: 12, viewportWidth: 100, wantTruncated: false, wantNameInRow: "user-service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{width: tt.viewportWidth + 8}
			m.servicesViewport.Width = tt.viewportWidth
			service := &ServiceState{Name: tt.serviceName, Status: StatusReady, LogEnabled: true}

			result := m.renderServiceRow(service, false, tt.maxNameLen)

			assert.Contains(t, result, tt.wantNameInRow)

			if tt.wantTruncated {
				assert.Contains(t, result, "…")
			}
		})
	}
}

func Test_RenderServiceRow_ColumnAlignment(t *testing.T) {
	m := Model{width: 120}
	m.servicesViewport.Width = 112

	service1 := &ServiceState{Name: "api", Status: StatusReady, LogEnabled: true}
	service2 := &ServiceState{Name: "user-management-service", Status: StatusStarting, LogEnabled: false}

	row1 := m.renderServiceRow(service1, false, 25)
	row2 := m.renderServiceRow(service2, false, 25)

	assert.Contains(t, row1, "[x] api")
	assert.Contains(t, row1, "Ready")
	assert.Contains(t, row2, "[ ] user-management-service")
	assert.Contains(t, row2, "Starting")
}

func Test_RenderServiceRow_SelectedIndicator(t *testing.T) {
	m := Model{width: 100}
	m.servicesViewport.Width = 92

	service := &ServiceState{Name: "api", Status: StatusReady, LogEnabled: true}

	notSelected := m.renderServiceRow(service, false, 20)
	selected := m.renderServiceRow(service, true, 20)

	assert.Contains(t, notSelected, "  [x]")
	assert.Contains(t, selected, "▸ [x]")
}

func Test_RenderTier_Spacing(t *testing.T) {
	m := Model{
		services: map[string]*ServiceState{
			"api": {Name: "api", Status: StatusReady, LogEnabled: true},
			"db":  {Name: "db", Status: StatusReady, LogEnabled: true},
		},
		width: 100,
	}
	m.servicesViewport.Width = 92

	tier := TierView{Name: "platform", Services: []string{"api", "db"}}
	currentIdx := 0

	firstTier := m.renderTier(tier, &currentIdx, 20, true)
	currentIdx = 0
	secondTier := m.renderTier(tier, &currentIdx, 20, false)

	assert.True(t, firstTier[0] != '\n', "first tier should not start with newline")
	assert.True(t, secondTier[0] == '\n', "non-first tier should start with newline")
	assert.Contains(t, firstTier, "platform\n")
	assert.Contains(t, secondTier, "platform\n")
}

func Test_RenderTier_ServiceCount(t *testing.T) {
	m := Model{
		services: map[string]*ServiceState{
			"api": {Name: "api", Status: StatusReady, LogEnabled: true},
			"db":  {Name: "db", Status: StatusReady, LogEnabled: true},
			"web": {Name: "web", Status: StatusReady, LogEnabled: true},
		},
		width: 100,
	}
	m.servicesViewport.Width = 92

	tier := TierView{Name: "platform", Services: []string{"api", "db", "web"}}
	currentIdx := 0

	result := m.renderTier(tier, &currentIdx, 20, true)

	assert.Equal(t, 3, currentIdx)
	assert.Contains(t, result, "api")
	assert.Contains(t, result, "db")
	assert.Contains(t, result, "web")
}
