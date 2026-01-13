package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	apperrors "fuku/internal/app/errors"
)

// simplifyErrorMessage returns a user-friendly short error message
func simplifyErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	switch {
	case errors.Is(err, apperrors.ErrMaxRetriesExceeded):
		return "max retries exceeded"
	case errors.Is(err, apperrors.ErrProcessExited):
		return "process exited"
	case errors.Is(err, apperrors.ErrReadinessTimeout):
		return "readiness timeout"
	case errors.Is(err, apperrors.ErrFailedToStartCommand):
		return "failed to start"
	case errors.Is(err, apperrors.ErrServiceNotFound):
		return "service not found"
	case errors.Is(err, apperrors.ErrServiceDirectoryNotExist):
		return "directory not found"
	default:
		return err.Error()
	}
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
