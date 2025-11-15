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
