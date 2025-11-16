package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// LogEntry represents a single log line for the LogView
type LogEntry struct {
	Timestamp time.Time
	Service   string
	Tier      string
	Stream    string
	Message   string
}

// LogView provides an interface for log viewing and filtering
type LogView interface {
	// HandleLog adds a log entry
	HandleLog(entry LogEntry)
	// IsEnabled returns whether logs are enabled for a service
	IsEnabled(service string) bool
	// SetEnabled updates the visibility state for a service
	SetEnabled(service string, enabled bool)
	// EnableAll enables logs for all specified services
	EnableAll(services []string)

	// View returns the rendered logs view
	View() string
	// SetSize updates the viewport dimensions
	SetSize(width, height int)
	// ToggleAutoscroll toggles autoscroll mode
	ToggleAutoscroll()
	// Autoscroll returns the current autoscroll state
	Autoscroll() bool
	// HandleKey processes keyboard input for scrolling
	HandleKey(msg tea.KeyMsg) tea.Cmd
}
