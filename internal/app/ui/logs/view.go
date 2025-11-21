package logs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"fuku/internal/app/ui/components"
)

// truncateServiceName truncates a service name to fit within maxWidth display columns.
// It preserves UTF-8 correctness by truncating at rune boundaries and uses display width
// (not byte or rune count) to ensure proper visual fitting.
func truncateServiceName(serviceName string, maxWidth int) string {
	currentWidth := lipgloss.Width(serviceName)
	if currentWidth <= maxWidth {
		return serviceName
	}

	ellipsis := "…"
	ellipsisWidth := lipgloss.Width(ellipsis)
	targetWidth := maxWidth - ellipsisWidth

	if targetWidth <= 0 {
		return ellipsis
	}

	runes := []rune(serviceName)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i])
		if lipgloss.Width(candidate) <= targetWidth {
			return candidate + ellipsis
		}
	}

	return ellipsis
}

// View returns the rendered logs view
func (m Model) View() string {
	if len(strings.TrimSpace(m.viewport.View())) == 0 {
		return components.EmptyStateStyle.Render("No logs enabled. Press 'space' to toggle service logs. Press 'tab' to return to services view.")
	}

	return m.viewport.View()
}

func (m *Model) updateContent() {
	viewportWidth := m.viewport.Width
	if viewportWidth <= 0 {
		viewportWidth = components.DefaultViewportWidth
	}

	// Check if width changed (requires full rebuild for rewrapping)
	if m.lastWidth != viewportWidth {
		m.invalidateCache()
		m.lastWidth = viewportWidth
	}

	// Capture scroll position before SetContent resets it
	oldYOffset := m.viewport.YOffset

	// Render only new entries (incremental rendering)
	for i := m.lastRendered + 1; i < len(m.entries); i++ {
		entry := m.entries[i]

		var builder strings.Builder

		if m.filter.IsEnabled(entry.Service) {
			m.renderEntry(&builder, entry, viewportWidth)
		}

		m.renderedLines = append(m.renderedLines, builder.String())
	}

	m.lastRendered = len(m.entries) - 1

	// Build final content from cached rendered lines
	var content strings.Builder

	for i, entry := range m.entries {
		if !m.filter.IsEnabled(entry.Service) {
			continue
		}

		if i < len(m.renderedLines) && m.renderedLines[i] != "" {
			content.WriteString(m.renderedLines[i])
		}
	}

	m.viewport.SetContent(content.String())

	if m.autoscroll {
		m.viewport.GotoBottom()
	} else {
		// Preserve scroll position when not autoscrolling
		maxYOffset := m.viewport.TotalLineCount() - m.viewport.Height
		if maxYOffset < 0 {
			maxYOffset = 0
		}

		if oldYOffset > maxYOffset {
			m.viewport.YOffset = maxYOffset
		} else {
			m.viewport.YOffset = oldYOffset
		}
	}
}

func (m *Model) renderEntry(builder *strings.Builder, entry Entry, viewportWidth int) {
	serviceName := truncateServiceName(entry.Service, components.LogServiceNameMaxWidth)

	service := components.ServiceNameStyle.Render(serviceName)
	divider := components.TimestampStyle.Render("·")

	prefix := fmt.Sprintf("%s %s ", service, divider)
	prefixLen := lipgloss.Width(prefix)

	continuationPrefixLen := lipgloss.Width(" │ ")

	message := strings.TrimRight(entry.Message, "\n\r")
	message = highlightLogLevel(message)

	lines := strings.Split(message, "\n")

	var allWrappedLines []string

	firstLineMaxWidth := viewportWidth - prefixLen
	if firstLineMaxWidth < components.LogMessageMinWidth {
		firstLineMaxWidth = components.LogMessageMinWidth
	}

	continuationMaxWidth := viewportWidth - continuationPrefixLen
	if continuationMaxWidth < components.LogMessageMinWidth {
		continuationMaxWidth = components.LogMessageMinWidth
	}

	for lineIdx, line := range lines {
		if lineIdx == 0 {
			wrappedLines := wrapText(line, firstLineMaxWidth)
			if len(wrappedLines) == 0 {
				continue
			}

			allWrappedLines = append(allWrappedLines, wrappedLines[0])

			if len(wrappedLines) > 1 {
				remainder := strings.Join(wrappedLines[1:], " ")
				rewrappedLines := wrapText(remainder, continuationMaxWidth)
				allWrappedLines = append(allWrappedLines, rewrappedLines...)
			}
		} else {
			wrappedLines := wrapText(line, continuationMaxWidth)
			allWrappedLines = append(allWrappedLines, wrappedLines...)
		}
	}

	for i, wrappedLine := range allWrappedLines {
		isFirstLine := i == 0
		isLastLine := i == len(allWrappedLines)-1

		var linePrefix string

		switch {
		case isFirstLine:
			linePrefix = prefix
		case isLastLine:
			linePrefix = " └ "
		default:
			linePrefix = " │ "
		}

		builder.WriteString(linePrefix)
		builder.WriteString(wrappedLine)
		builder.WriteRune('\n')
	}
}
