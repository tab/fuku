package logs

import (
	"strings"
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
	entries           []Entry  // Ring buffer: fixed-size array
	renderedLines     []string // Ring buffer: pre-rendered line cache
	head              int      // Index of oldest entry in ring
	tail              int      // Index where next entry will be written
	count             int      // Number of active entries in ring
	maxSize           int      // Maximum buffer size (LogBufferSize)
	lastRenderedIndex int      // Logical index of last entry rendered and added to content (-1 if none)
	currentContent    string   // Current viewport content for append-only updates
	contentLines      []string // Ring-aligned rendered content lines for O(1) eviction
	contentHead       int      // Head pointer for contentLines ring
	contentCount      int      // Number of lines in contentLines
	filter            ui.LogFilter
	viewport          viewport.Model
	autoscroll        bool
	width             int
	height            int
	lastWidth         int  // Track width for change detection
	widthDirty        bool // True when width changed, triggers re-render
}

// NewModel creates a new logs model with its own filter
func NewModel() Model {
	maxSize := components.LogBufferSize

	return Model{
		entries:           make([]Entry, maxSize),
		renderedLines:     make([]string, maxSize),
		head:              0,
		tail:              0,
		count:             0,
		maxSize:           maxSize,
		lastRenderedIndex: -1,
		currentContent:    "",
		contentLines:      make([]string, maxSize),
		contentHead:       0,
		contentCount:      0,
		filter:            ui.NewLogFilter(),
		viewport:          viewport.New(0, 0),
		autoscroll:        false,
		lastWidth:         0,
		widthDirty:        false,
	}
}

// ringIndex converts a logical index (0 to count-1) to physical ring buffer index
func (m *Model) ringIndex(logicalIndex int) int {
	return (m.head + logicalIndex) % m.maxSize
}

// addEntry adds a new entry to the ring buffer and renders it immediately
func (m *Model) addEntry(entry Entry) {
	viewportWidth := m.viewport.Width
	if viewportWidth <= 0 {
		viewportWidth = components.DefaultViewportWidth
	}

	m.entries[m.tail] = entry

	var builder strings.Builder
	m.renderEntryWithWidth(&builder, entry, viewportWidth)
	rendered := builder.String()

	m.renderedLines[m.tail] = rendered

	contentTail := (m.contentHead + m.contentCount) % m.maxSize
	m.contentLines[contentTail] = rendered

	m.tail = (m.tail + 1) % m.maxSize

	evicted := false

	if m.count < m.maxSize {
		m.count++
		m.contentCount++
	} else {
		evictedEntry := m.entries[m.head]
		evictedRendered := m.renderedLines[m.head]

		if m.filter.IsEnabled(evictedEntry.Service) && evictedRendered != "" {
			// Drop the evicted rendered prefix from currentContent without a full rebuild
			if len(evictedRendered) <= len(m.currentContent) {
				m.currentContent = m.currentContent[len(evictedRendered):]
			} else {
				m.currentContent = ""
			}
		}

		m.head = (m.head + 1) % m.maxSize
		m.contentHead = (m.contentHead + 1) % m.maxSize
		// contentCount stays at maxSize once full

		evicted = true

		if m.lastRenderedIndex >= 0 {
			m.lastRenderedIndex--
			if m.lastRenderedIndex < 0 {
				m.lastRenderedIndex = 0
			}
		}
	}

	// When evicted, keep appending path valid; rebuild only if we dropped all content
	if evicted && m.currentContent == "" {
		m.lastRenderedIndex = -1
	}
}

// getEntry retrieves an entry by logical index (0 to count-1)
func (m *Model) getEntry(logicalIndex int) Entry {
	return m.entries[m.ringIndex(logicalIndex)]
}

// getRenderedLine retrieves the rendered line for a logical index
func (m *Model) getRenderedLine(logicalIndex int) string {
	return m.renderedLines[m.ringIndex(logicalIndex)]
}

// invalidateContent clears the content cache without clearing rendered lines
func (m *Model) invalidateContent() {
	m.lastRenderedIndex = -1
	m.currentContent = ""
}

// Clear resets the log buffer and viewport while preserving filter and autoscroll state
func (m *Model) Clear() {
	for i := 0; i < m.count; i++ {
		physicalIdx := m.ringIndex(i)
		m.entries[physicalIdx] = Entry{}
		m.renderedLines[physicalIdx] = ""
		m.contentLines[physicalIdx] = ""
	}

	m.head = 0
	m.tail = 0
	m.count = 0
	m.contentHead = 0
	m.contentCount = 0
	m.lastRenderedIndex = -1
	m.currentContent = ""
	m.widthDirty = false
	m.viewport.SetContent("")
	m.viewport.YOffset = 0
}

// IsEnabled returns whether logs are enabled for a service
func (m Model) IsEnabled(service string) bool {
	return m.filter.IsEnabled(service)
}

// SetEnabled updates the visibility state for a service
func (m *Model) SetEnabled(service string, enabled bool) {
	m.filter.Set(service, enabled)
	m.invalidateContent()
	m.updateContent()
}

// ToggleAll toggles all services (if all enabled -> disable all, otherwise -> enable all)
func (m *Model) ToggleAll(services []string) {
	m.filter.ToggleAll(services)
	m.invalidateContent()
	m.updateContent()
}

// SetSize updates the viewport dimensions
func (m *Model) SetSize(width, height int) {
	if m.lastWidth != 0 && m.lastWidth != width {
		m.widthDirty = true
	}

	m.width = width
	m.height = height
	m.lastWidth = width
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

	m.addEntry(logEntry)
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
		m.addEntry(Entry(logMsg))
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

// buildContent builds the full content string from filtered rendered lines (for testing)
func (m *Model) buildContent() string {
	var content strings.Builder

	for i := 0; i < m.count; i++ {
		entry := m.getEntry(i)

		if !m.filter.IsEnabled(entry.Service) {
			continue
		}

		rendered := m.getRenderedLine(i)
		if rendered != "" {
			content.WriteString(rendered)
		}
	}

	return content.String()
}
