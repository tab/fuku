package services

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

// truncateErrorMessage truncates an error message to fit within availableWidth display columns
func truncateErrorMessage(errorText string, availableWidth int) string {
	if availableWidth <= 0 {
		return ""
	}

	prefix := " ("
	suffix := ")"
	formatted := fmt.Sprintf("%s%s%s", prefix, errorText, suffix)

	currentWidth := lipgloss.Width(formatted)
	if currentWidth <= availableWidth {
		return formatted
	}

	wrapperWidth := lipgloss.Width(prefix) + lipgloss.Width(suffix)
	ellipsis := "…"
	ellipsisWidth := lipgloss.Width(ellipsis)

	minWidth := wrapperWidth + ellipsisWidth
	if availableWidth < minWidth {
		if availableWidth >= ellipsisWidth {
			return ellipsis
		}

		return ""
	}

	targetWidth := availableWidth - wrapperWidth - ellipsisWidth

	if targetWidth <= 0 {
		return fmt.Sprintf("%s%s%s", prefix, ellipsis, suffix)
	}

	runes := []rune(errorText)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i])
		if lipgloss.Width(candidate) <= targetWidth {
			return fmt.Sprintf("%s%s%s%s", prefix, candidate, ellipsis, suffix)
		}
	}

	return fmt.Sprintf("%s%s%s", prefix, ellipsis, suffix)
}

// padServiceName pads a service name to maxWidth using display width (not rune count)
func padServiceName(serviceName string, maxWidth int) string {
	currentWidth := lipgloss.Width(serviceName)
	if currentWidth >= maxWidth {
		return serviceName
	}

	padding := maxWidth - currentWidth

	return serviceName + strings.Repeat(" ", padding)
}
