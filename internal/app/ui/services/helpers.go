package services

import (
	"fmt"
	"strings"

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

// truncateErrorMessage truncates an error message to fit within availableWidth display columns.
// It formats the error as " (errorText)" and truncates if needed, preserving UTF-8 correctness.
// If there's insufficient space for the wrapper, returns just an ellipsis or empty string.
func truncateErrorMessage(errorText string, availableWidth int) string {
	if availableWidth <= 0 {
		return ""
	}

	// Format error with wrapper: " (error text here)"
	prefix := " ("
	suffix := ")"
	formatted := fmt.Sprintf("%s%s%s", prefix, errorText, suffix)

	currentWidth := lipgloss.Width(formatted)
	if currentWidth <= availableWidth {
		return formatted
	}

	// Need to truncate - calculate space for error text inside wrapper
	wrapperWidth := lipgloss.Width(prefix) + lipgloss.Width(suffix)
	ellipsis := "…"
	ellipsisWidth := lipgloss.Width(ellipsis)

	// Check if we have room for wrapper + ellipsis
	minWidth := wrapperWidth + ellipsisWidth
	if availableWidth < minWidth {
		// Not enough room for wrapper, return just ellipsis if it fits
		if availableWidth >= ellipsisWidth {
			return ellipsis
		}

		return ""
	}

	// Truncate error text to fit: availableWidth = prefix + errorText + ellipsis + suffix
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

// padServiceName pads a service name to maxWidth using display width (not rune count).
// This ensures proper column alignment for service names containing wide characters
// (emoji, CJK, etc.) or multi-byte UTF-8 characters.
func padServiceName(serviceName string, maxWidth int) string {
	currentWidth := lipgloss.Width(serviceName)
	if currentWidth >= maxWidth {
		return serviceName
	}

	padding := maxWidth - currentWidth

	return serviceName + strings.Repeat(" ", padding)
}
