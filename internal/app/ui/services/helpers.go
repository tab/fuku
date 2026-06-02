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
	mainWidth, _ := m.panelWidths()
	contentWidth := mainWidth - components.PanelInnerPadding - components.RowHorizontalPadding

	if m.asideVisible() {
		preferred := min(m.longestServiceNameWidth(), components.ServiceNameWidthMedium)
		m.ui.layout = components.ComputeCompactLayout(contentWidth, preferred)

		return m
	}

	preferred := components.PreferredNameTextWidth(m.longestServiceNameWidth())
	m.ui.layout = components.ComputeTableLayout(contentWidth, preferred, components.MetricFullColumnCount)

	return m
}

// computeAsideWidths returns auto-fit widths for the split layout, sizing main to the longest service name (capped at ServiceNameWidthMedium so a single outlier service name cannot keep stealing aside space) plus a status column, and donating the rest to aside
func (m Model) computeAsideWidths() (mainWidth, asideWidth int) {
	longest := min(m.longestServiceNameWidth(), components.ServiceNameWidthMedium)
	needed := longest +
		components.IndicatorColumnWidth +
		components.ServiceNameTrailingGap +
		components.StatusCompactWidth +
		components.PanelInnerPadding +
		components.RowHorizontalPadding

	mainWidth = max(needed, components.AsideMinMainWidth)
	asideWidth = m.ui.width - mainWidth

	return mainWidth, asideWidth
}

// panelWidths returns the widths of the main panel and aside panel based on aside state
func (m Model) panelWidths() (mainWidth, asideWidth int) {
	if !m.state.asideOpen {
		return m.ui.width, 0
	}

	mainWidth, asideWidth = m.computeAsideWidths()
	if asideWidth < components.AsideMinWidth || mainWidth < components.AsideMinMainWidth {
		return m.ui.width, 0
	}

	return mainWidth, asideWidth
}

// canShowAside reports whether the current terminal width can fit the split layout
func (m Model) canShowAside() bool {
	mainWidth, asideWidth := m.computeAsideWidths()

	return asideWidth >= components.AsideMinWidth && mainWidth >= components.AsideMinMainWidth
}

// asideVisible reports whether the aside should be rendered for the current width
func (m Model) asideVisible() bool {
	_, asideWidth := m.panelWidths()
	return asideWidth > 0
}

// servicesPanelBorderStyle returns the active primary border when services has focus and the muted border when the aside has focus
func (m Model) servicesPanelBorderStyle() lipgloss.Style {
	if m.state.asideOpen && m.state.asideFocused {
		return components.PanelMutedBorderStyle
	}

	return components.PanelBorderStyle
}

// asidePanelBorderStyle returns the active primary border when aside has focus and the muted border when services has focus
func (m Model) asidePanelBorderStyle() lipgloss.Style {
	if m.state.asideFocused {
		return components.PanelBorderStyle
	}

	return components.PanelMutedBorderStyle
}

// recomputeViewport updates the services and aside viewport dimensions to match the panel split
func (m *Model) recomputeViewport() {
	mainWidth, asideWidth := m.panelWidths()

	panelHeight := max(m.ui.height-components.PanelHeightPadding, components.PanelMinHeight)
	contentHeight := panelHeight - components.PanelBorderHeight

	m.ui.servicesViewport.SetWidth(mainWidth - components.PanelInnerPadding)
	m.ui.servicesViewport.SetHeight(contentHeight)

	m.ui.asideViewport.SetWidth(max(asideWidth-components.PanelInnerPadding, 0))
	m.ui.asideViewport.SetHeight(contentHeight)
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
