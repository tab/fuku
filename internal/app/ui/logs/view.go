package logs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"fuku/internal/app/ui/components"
)

// truncateServiceName truncates a service name to fit within maxWidth display columns
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

	if m.widthDirty {
		m.widthDirty = false
		m.rebuildAfterWidthChange(viewportWidth)

		return
	}

	oldYOffset := m.viewport.YOffset
	needsRebuild := m.lastRenderedIndex == -1

	if needsRebuild {
		m.rebuildContent()
	} else {
		if m.lastRenderedIndex >= m.count-1 {
			return
		}

		var newLines strings.Builder

		for i := m.lastRenderedIndex + 1; i < m.count; i++ {
			entry := m.getEntry(i)

			if !m.filter.IsEnabled(entry.Service) {
				continue
			}

			contentIdx := (m.contentHead + i) % m.maxSize
			rendered := m.contentLines[contentIdx]

			if rendered != "" {
				newLines.WriteString(rendered)
			}
		}

		if newLines.Len() > 0 {
			m.currentContent += newLines.String()
			m.viewport.SetContent(m.currentContent)
		}

		m.lastRenderedIndex = m.count - 1
	}

	if m.autoscroll {
		m.viewport.GotoBottom()
	} else {
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

// rebuildContent rebuilds currentContent from contentLines without re-rendering
func (m *Model) rebuildContent() {
	var content strings.Builder

	for i := 0; i < m.count; i++ {
		entry := m.getEntry(i)

		if !m.filter.IsEnabled(entry.Service) {
			continue
		}

		contentIdx := (m.contentHead + i) % m.maxSize
		rendered := m.contentLines[contentIdx]

		if rendered != "" {
			content.WriteString(rendered)
		}
	}

	m.currentContent = content.String()
	m.viewport.SetContent(m.currentContent)
	m.lastRenderedIndex = m.count - 1
}

// rebuildAfterWidthChange re-renders all entries with new width and rebuilds content
func (m *Model) rebuildAfterWidthChange(viewportWidth int) {
	for i := 0; i < m.count; i++ {
		entry := m.getEntry(i)

		var builder strings.Builder
		m.renderEntry(&builder, entry, viewportWidth)
		rendered := builder.String()

		physicalIdx := m.ringIndex(i)
		m.renderedLines[physicalIdx] = rendered

		contentIdx := (m.contentHead + i) % m.maxSize
		m.contentLines[contentIdx] = rendered
	}

	var content strings.Builder

	for i := 0; i < m.count; i++ {
		entry := m.getEntry(i)

		if !m.filter.IsEnabled(entry.Service) {
			continue
		}

		contentIdx := (m.contentHead + i) % m.maxSize
		rendered := m.contentLines[contentIdx]

		if rendered != "" {
			content.WriteString(rendered)
		}
	}

	m.currentContent = content.String()
	m.viewport.SetContent(m.currentContent)
	m.lastRenderedIndex = m.count - 1
}

// renderEntryWithWidth is called from model.go to render entries on insert
func (m *Model) renderEntryWithWidth(builder *strings.Builder, entry Entry, viewportWidth int) {
	m.renderEntry(builder, entry, viewportWidth)
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
