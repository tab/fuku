package logs

import (
	"fmt"
	"strings"
)

// View returns the rendered logs view
func (m Model) View() string {
	if len(strings.TrimSpace(m.viewport.View())) == 0 {
		return emptyStateStyle.Render("No logs enabled. Press 'space' to toggle service logs. Press 'l' to return to services view.")
	}

	return m.viewport.View()
}

func (m *Model) updateContent() {
	var logLines string

	viewportWidth := m.viewport.Width
	if viewportWidth <= 0 {
		viewportWidth = 80
	}

	for _, entry := range m.entries {
		if !m.filter.IsEnabled(entry.Service) {
			continue
		}

		serviceName := entry.Service
		if len(serviceName) > maxServiceNameWidth {
			serviceName = serviceName[:maxServiceNameWidth] + "…"
		}

		service := serviceNameStyle.Render(serviceName)
		divider := timestampStyle.Render("·")

		prefix := fmt.Sprintf("%s %s ", service, divider)
		prefixLen := len(serviceName) + prefixSpacing

		message := strings.TrimRight(entry.Message, "\n\r")

		messageWidth := viewportWidth - prefixLen
		if messageWidth < minMessageWidth {
			messageWidth = minMessageWidth
		}

		wrappedMessage := logMessageStyle.Width(messageWidth).Render(message)

		lines := strings.Split(wrappedMessage, "\n")
		for i, line := range lines {
			linePrefix := prefix
			if i > 0 {
				linePrefix = "└ "
			}

			logLines += fmt.Sprintf("%s%s\n", linePrefix, line)
		}
	}

	m.viewport.SetContent(logLines)

	if m.autoscroll {
		m.viewport.GotoBottom()
	}
}
