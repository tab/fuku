package logs

import (
	"time"

	"github.com/charmbracelet/bubbles/viewport"

	"fuku/internal/app/ui"
)

// Entry represents a single log line
type Entry struct {
	Timestamp time.Time
	Service   string
	Tier      string
	Stream    string
	Message   string
}

// Model represents the logs viewer state
type Model struct {
	entries    []Entry
	maxSize    int
	filter     ui.LogFilter
	viewport   viewport.Model
	autoscroll bool
	width      int
	height     int
}

// NewModel creates a new logs model
func NewModel(filter ui.LogFilter) Model {
	return Model{
		entries:    make([]Entry, 0),
		maxSize:    maxEntries,
		filter:     filter,
		viewport:   viewport.New(0, 0),
		autoscroll: false,
	}
}

// SetSize updates the viewport dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width - viewportWidthPadding
	m.viewport.Height = height
}

// ToggleAutoscroll toggles autoscroll mode
func (m *Model) ToggleAutoscroll() {
	m.autoscroll = !m.autoscroll
	if m.autoscroll {
		m.viewport.GotoBottom()
	}
}

// Autoscroll returns the current autoscroll state
func (m Model) Autoscroll() bool {
	return m.autoscroll
}

// AddEntry adds a new log entry
func (m *Model) AddEntry(entry Entry) {
	m.entries = append(m.entries, entry)

	if len(m.entries) > m.maxSize {
		m.entries = m.entries[len(m.entries)-m.maxSize:]
	}

	m.updateContent()
}

// Viewport returns the viewport for external updates
func (m *Model) Viewport() *viewport.Model {
	return &m.viewport
}
