package logs

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"fuku/internal/app/ui"
	"fuku/internal/app/ui/components"
)

func Test_NewModel(t *testing.T) {
	model := NewModel()
	assert.NotNil(t, model.filter)
	assert.Equal(t, components.LogBufferSize, model.maxSize)
	assert.False(t, model.autoscroll)
	assert.Empty(t, model.entries)
}

func Test_Model_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(m *Model)
		service  string
		expected bool
	}{
		{name: "Not enabled by default", setup: func(m *Model) {}, service: "api", expected: false},
		{name: "Enabled after SetEnabled", setup: func(m *Model) { m.SetEnabled("api", true) }, service: "api", expected: true},
		{name: "Disabled after SetEnabled", setup: func(m *Model) { m.SetEnabled("api", false) }, service: "api", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel()
			tt.setup(&model)
			assert.Equal(t, tt.expected, model.IsEnabled(tt.service))
		})
	}
}

func Test_Model_SetEnabled(t *testing.T) {
	model := NewModel()
	model.SetEnabled("api", true)
	assert.True(t, model.IsEnabled("api"))
	model.SetEnabled("api", false)
	assert.False(t, model.IsEnabled("api"))
}

func Test_Model_ToggleAll(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(m *Model)
		services []string
		expected map[string]bool
	}{
		{
			name:     "Toggle all from disabled to enabled",
			setup:    func(m *Model) {},
			services: []string{"api", "web", "db"},
			expected: map[string]bool{"api": true, "web": true, "db": true},
		},
		{
			name: "Toggle all from enabled to disabled",
			setup: func(m *Model) {
				for _, s := range []string{"api", "web", "db"} {
					m.SetEnabled(s, true)
				}
			},
			services: []string{"api", "web", "db"},
			expected: map[string]bool{"api": false, "web": false, "db": false},
		},
		{
			name:     "Toggle with some enabled to all enabled",
			setup:    func(m *Model) { m.SetEnabled("api", true) },
			services: []string{"api", "web", "db"},
			expected: map[string]bool{"api": true, "web": true, "db": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel()
			tt.setup(&model)
			model.ToggleAll(tt.services)

			for svc, expectedEnabled := range tt.expected {
				assert.Equal(t, expectedEnabled, model.IsEnabled(svc))
			}
		})
	}
}

func Test_Model_HandleLog(t *testing.T) {
	tests := []struct {
		name          string
		entries       int
		expectedCount int
	}{
		{name: "Single entry", entries: 1, expectedCount: 1},
		{name: "Multiple entries", entries: 5, expectedCount: 5},
		{name: "Max entries truncates", entries: components.LogBufferSize + 10, expectedCount: components.LogBufferSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel()
			model.SetSize(100, 50)

			for i := 0; i < tt.entries; i++ {
				model.HandleLog(ui.LogEntry{Timestamp: time.Now(), Service: "api", Tier: "tier1", Stream: "STDOUT", Message: "test"})
			}

			assert.Equal(t, tt.expectedCount, len(model.entries))
		})
	}
}

func Test_Model_SetSize(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{name: "Normal size", width: 100, height: 50},
		{name: "Small size", width: 40, height: 20},
		{name: "Large size", width: 200, height: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel()
			model.SetSize(tt.width, tt.height)
			assert.Equal(t, tt.width, model.width)
			assert.Equal(t, tt.height, model.height)
		})
	}
}

func Test_Model_ToggleAutoscroll(t *testing.T) {
	model := NewModel()
	assert.False(t, model.Autoscroll())
	model.ToggleAutoscroll()
	assert.True(t, model.Autoscroll())
	model.ToggleAutoscroll()
	assert.False(t, model.Autoscroll())
}

func Test_Model_HandleKey(t *testing.T) {
	model := NewModel()
	model.SetSize(100, 50)

	msg := tea.KeyMsg{Type: tea.KeyUp}
	cmd := model.HandleKey(msg)

	assert.Nil(t, cmd)
}

func Test_Model_View(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(m *Model)
		contains string
	}{
		{name: "Empty view shows message", setup: func(m *Model) { m.SetSize(100, 50) }, contains: "No logs"},
		{
			name: "View with entries",
			setup: func(m *Model) {
				m.SetSize(100, 50)
				m.SetEnabled("api", true)
				m.HandleLog(ui.LogEntry{Timestamp: time.Now(), Service: "api", Tier: "tier1", Stream: "STDOUT", Message: "test message"})
			},
			contains: "test message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel()
			tt.setup(&model)
			view := model.View()
			assert.Contains(t, view, tt.contains)
		})
	}
}

func Test_Model_FilterRerender(t *testing.T) {
	model := NewModel()
	model.SetSize(80, 20)

	// Service logs arrive while disabled
	model.HandleLog(ui.LogEntry{Timestamp: time.Now(), Service: "api", Tier: "tier1", Stream: "STDOUT", Message: "first"})
	assert.Contains(t, model.View(), "No logs enabled")

	// Enabling should immediately render buffered entries
	model.SetEnabled("api", true)
	assert.Contains(t, model.View(), "first")

	// Disabling should immediately hide them
	model.SetEnabled("api", false)
	assert.Contains(t, model.View(), "No logs enabled")
}
