package logs

import (
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"fuku/internal/app/ui"
	"fuku/internal/app/ui/components"
)

type logEntryMsg ui.LogEntry

// Entry represents a single log line
type Entry struct {
	Timestamp time.Time
	Service   string
	Tier      string
	Stream    string
	Message   string
}

// Model represents the logs viewer state and implements ui.LogView
type Model struct {
	entries    []Entry
	maxSize    int
	filter     ui.LogFilter
	viewport   viewport.Model
	autoscroll bool
	width      int
	height     int
}

// NewModel creates a new logs model with its own filter
func NewModel() Model {
	return Model{
		entries:    make([]Entry, 0),
		maxSize:    components.LogBufferSize,
		filter:     ui.NewLogFilter(),
		viewport:   viewport.New(0, 0),
		autoscroll: false,
	}
}

// IsEnabled returns whether logs are enabled for a service
func (m Model) IsEnabled(service string) bool {
	return m.filter.IsEnabled(service)
}

// SetEnabled updates the visibility state for a service
func (m *Model) SetEnabled(service string, enabled bool) {
	m.filter.Set(service, enabled)
	m.updateContent()
}

// ToggleAll toggles all services (if all enabled -> disable all, otherwise -> enable all)
func (m *Model) ToggleAll(services []string) {
	m.filter.ToggleAll(services)
	m.updateContent()
}

// SetSize updates the viewport dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height
	m.updateContent()
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

// HandleLog adds a log entry (implements ui.LogView)
func (m *Model) HandleLog(entry ui.LogEntry) {
	logEntry := Entry{
		Timestamp: entry.Timestamp,
		Service:   entry.Service,
		Tier:      entry.Tier,
		Stream:    entry.Stream,
		Message:   entry.Message,
	}

	m.entries = append(m.entries, logEntry)

	if len(m.entries) > m.maxSize {
		m.entries = m.entries[len(m.entries)-m.maxSize:]
	}

	m.updateContent()
}

// HandleKey processes keyboard input for scrolling
func (m *Model) HandleKey(msg tea.KeyMsg) tea.Cmd {
	oldYOffset := m.viewport.YOffset

	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)

	if m.viewport.YOffset != oldYOffset && m.autoscroll {
		atBottom := m.viewport.YOffset >= m.viewport.TotalLineCount()-m.viewport.Height
		if !atBottom {
			m.autoscroll = false
		}
	}

	return cmd
}

// Update handles Bubble Tea messages
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	if logMsg, ok := msg.(logEntryMsg); ok {
		m.entries = append(m.entries, Entry(logMsg))

		if len(m.entries) > m.maxSize {
			m.entries = m.entries[len(m.entries)-m.maxSize:]
		}

		m.updateContent()
	}

	return m, nil
}

// SendLog creates a command that sends a log entry message
func SendLog(entry ui.LogEntry) tea.Cmd {
	return func() tea.Msg {
		return logEntryMsg(entry)
	}
}
