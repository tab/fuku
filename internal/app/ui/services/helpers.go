package services

import (
	"github.com/charmbracelet/lipgloss"
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
