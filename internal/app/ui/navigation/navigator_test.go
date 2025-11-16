package navigation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewNavigator(t *testing.T) {
	nav := NewNavigator()
	assert.NotNil(t, nav)
	assert.Equal(t, ViewServices, nav.CurrentView())
}

func Test_Navigator_CurrentView(t *testing.T) {
	nav := NewNavigator()
	assert.Equal(t, ViewServices, nav.CurrentView())
}

func Test_Navigator_SwitchTo(t *testing.T) {
	tests := []struct {
		name     string
		view     View
		expected View
	}{
		{name: "Switch to logs", view: ViewLogs, expected: ViewLogs},
		{name: "Switch to services", view: ViewServices, expected: ViewServices},
		{name: "Switch back to logs", view: ViewLogs, expected: ViewLogs},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav := NewNavigator()
			nav.SwitchTo(tt.view)
			assert.Equal(t, tt.expected, nav.CurrentView())
		})
	}
}

func Test_Navigator_Toggle(t *testing.T) {
	tests := []struct {
		name          string
		initialView   View
		expectedAfter View
	}{
		{name: "Toggle from services to logs", initialView: ViewServices, expectedAfter: ViewLogs},
		{name: "Toggle from logs to services", initialView: ViewLogs, expectedAfter: ViewServices},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav := NewNavigator()
			nav.SwitchTo(tt.initialView)
			nav.Toggle()
			assert.Equal(t, tt.expectedAfter, nav.CurrentView())
		})
	}
}

func Test_View_String(t *testing.T) {
	tests := []struct {
		name     string
		view     View
		expected string
	}{
		{name: "Services view", view: ViewServices, expected: "services"},
		{name: "Logs view", view: ViewLogs, expected: "logs"},
		{name: "Unknown view", view: View(99), expected: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.view.String())
		})
	}
}
