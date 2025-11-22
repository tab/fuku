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
	HandleLog(entry LogEntry)
	IsEnabled(service string) bool
	SetEnabled(service string, enabled bool)
	ToggleAll(services []string)
	View() string
	SetSize(width, height int)
	ToggleAutoscroll()
	Autoscroll() bool
	HandleKey(msg tea.KeyMsg) tea.Cmd
}
