package services

import (
	"charm.land/lipgloss/v2"

	"fuku/internal/app/errors"
	"fuku/internal/app/ui/components"
)

// longestServiceNameWidth returns the rendered cell width of the longest service name in state
func (m Model) longestServiceNameWidth() int {
	longest := 0

	for _, svc := range m.state.services {
		if w := lipgloss.Width(svc.Name); w > longest {
			longest = w
		}
	}

	return longest
}

// recomputeLayout updates the table layout based on current width and longest service name
func (m Model) recomputeLayout() Model {
	preferred := components.PreferredNameTextWidth(m.longestServiceNameWidth())
	contentWidth := m.ui.width - components.PanelInnerPadding - components.RowHorizontalPadding
	m.ui.layout = components.ComputeTableLayout(contentWidth, preferred)

	return m
}

// renderError returns a user-friendly error message
func renderError(err error) string {
	if err == nil {
		return ""
	}

	switch {
	case errors.Is(err, errors.ErrPortAlreadyInUse):
		return "port already in use"
	case errors.Is(err, errors.ErrMaxRetriesExceeded):
		return "max retries exceeded"
	case errors.Is(err, errors.ErrProcessExited):
		return "process exited"
	case errors.Is(err, errors.ErrReadinessTimeout):
		return "readiness timeout"
	case errors.Is(err, errors.ErrFailedToStartCommand):
		return "failed to start"
	case errors.Is(err, errors.ErrServiceNotFound):
		return "service not found"
	case errors.Is(err, errors.ErrServiceDirectoryNotExist):
		return "directory not found"
	default:
		return err.Error()
	}
}
