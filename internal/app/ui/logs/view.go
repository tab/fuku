package logs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"fuku/internal/app/ui/components"
)

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

	// Capture scroll position before SetContent resets it
	oldYOffset := m.viewport.YOffset

	var builder strings.Builder

	for _, entry := range m.entries {
		if !m.filter.IsEnabled(entry.Service) {
			continue
		}

		m.renderEntry(&builder, entry, viewportWidth)
	}

	m.viewport.SetContent(builder.String())

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
	serviceName := entry.Service
	if len(serviceName) > components.LogServiceNameMaxWidth {
		serviceName = serviceName[:components.LogServiceNameMaxWidth] + "…"
	}

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
