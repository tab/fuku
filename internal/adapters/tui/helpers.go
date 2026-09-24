package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"fuku/internal/adapters/terminal"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

// longestServiceNameWidth returns the rendered cell width of the longest service name in state
func (m Model) longestServiceNameWidth() int {
	longest := 0

	for _, svc := range m.snapshot.Services {
		if w := lipgloss.Width(svc.Name); w > longest {
			longest = w
		}
	}

	return longest
}

// recomputeLayout updates the table layout based on current width and longest service name
func (m Model) recomputeLayout() Model {
	mainWidth, _ := m.panelWidths()
	contentWidth := mainWidth - terminal.PanelInnerPadding - terminal.RowHorizontalPadding

	if m.asideVisible() {
		preferred := min(m.longestServiceNameWidth(), terminal.ServiceNameWidthMedium)
		m.ui.layout = terminal.ComputeCompactLayout(contentWidth, preferred)

		return m
	}

	preferred := terminal.PreferredNameTextWidth(m.longestServiceNameWidth())
	m.ui.layout = terminal.ComputeTableLayout(contentWidth, preferred, terminal.MetricFullColumnCount)

	return m
}

// computeAsideWidths sizes main to the capped longest name plus a status column and gives the aside the rest
func (m Model) computeAsideWidths() (mainWidth, asideWidth int) {
	longest := min(m.longestServiceNameWidth(), terminal.ServiceNameWidthMedium)
	needed := longest +
		terminal.IndicatorColumnWidth +
		terminal.ServiceNameTrailingGap +
		terminal.StatusCompactWidth +
		terminal.PanelInnerPadding +
		terminal.RowHorizontalPadding

	mainWidth = max(needed, terminal.AsideMinMainWidth)
	asideWidth = m.ui.width - mainWidth

	return mainWidth, asideWidth
}

// panelWidths returns the widths of the main panel and aside panel based on aside state
func (m Model) panelWidths() (mainWidth, asideWidth int) {
	if !m.state.asideOpen {
		return m.ui.width, 0
	}

	mainWidth, asideWidth = m.computeAsideWidths()
	if asideWidth < terminal.AsideMinWidth || mainWidth < terminal.AsideMinMainWidth {
		return m.ui.width, 0
	}

	return mainWidth, asideWidth
}

// canShowAside reports whether the current terminal width can fit the split layout
func (m Model) canShowAside() bool {
	mainWidth, asideWidth := m.computeAsideWidths()

	return asideWidth >= terminal.AsideMinWidth && mainWidth >= terminal.AsideMinMainWidth
}

// ensureAsideShowable closes and unfocuses the aside when the current layout can no longer fit the split
func (m *Model) ensureAsideShowable() {
	if !m.state.asideOpen {
		return
	}

	if m.canShowAside() {
		return
	}

	m.state.asideOpen = false
	m.state.asideFocused = false
}

// asideVisible reports whether the aside should be rendered for the current width
func (m Model) asideVisible() bool {
	_, asideWidth := m.panelWidths()
	return asideWidth > 0
}

// servicesPanelBorderStyle returns the primary border when services has focus and the muted border otherwise
func (m Model) servicesPanelBorderStyle() lipgloss.Style {
	if m.state.asideOpen && m.state.asideFocused {
		return m.theme.PanelMutedBorderStyle
	}

	return terminal.PanelBorderStyle
}

// asidePanelBorderStyle returns the primary border when the aside has focus and the muted border otherwise
func (m Model) asidePanelBorderStyle() lipgloss.Style {
	if m.state.asideFocused {
		return terminal.PanelBorderStyle
	}

	return m.theme.PanelMutedBorderStyle
}

// recomputeViewport updates the services and aside viewport dimensions to match the panel split
func (m *Model) recomputeViewport() {
	mainWidth, asideWidth := m.panelWidths()

	panelHeight := max(m.ui.height-terminal.PanelHeightPadding, terminal.PanelMinHeight)
	contentHeight := panelHeight - terminal.PanelBorderHeight

	m.ui.servicesViewport.SetWidth(mainWidth - terminal.PanelInnerPadding)
	m.ui.servicesViewport.SetHeight(contentHeight)

	m.ui.asideViewport.SetWidth(max(asideWidth-terminal.PanelInnerPadding, 0))
	m.ui.asideViewport.SetHeight(contentHeight)
}

// renderError returns a user-friendly error message (the snapshot carries the error as text, so sentinels match on it)
func renderError(text string) string {
	names := func(sentinel error) bool { return strings.Contains(text, sentinel.Error()) }

	switch {
	case names(contracts.ErrPortAlreadyInUse):
		return "port already in use"
	case names(contracts.ErrMaxRetriesExceeded):
		return "max retries exceeded"
	case names(contracts.ErrProcessExited):
		return "process exited"
	case names(contracts.ErrReadinessTimeout):
		return "readiness timeout"
	case names(contracts.ErrFailedToStartCommand):
		return "failed to start"
	case names(contracts.ErrServiceNotFound):
		return "service not found"
	case names(contracts.ErrServiceDirectoryNotExist):
		return "directory not found"
	default:
		return text
	}
}

// getUptime returns formatted uptime string for a service
func (m *Model) getUptime(service *model.Service) string {
	if service.Status.IsStartable() || service.Process.StartedAt.IsZero() || m.state.now.IsZero() {
		return ""
	}

	return formatElapsed(m.state.now.Sub(service.Process.StartedAt))
}

// formatCPU formats a CPU percentage value
func formatCPU(cpu float64) string {
	return fmt.Sprintf("%.1f%%", cpu)
}

// formatMEM formats a memory value in MB or GB
func formatMEM(mem float64) string {
	if mem < terminal.MBToGB {
		return fmt.Sprintf("%.0fMB", mem)
	}

	return fmt.Sprintf("%.1fGB", mem/terminal.MBToGB)
}

// getCPU returns formatted CPU usage for a service
func (m *Model) getCPU(service *model.Service) string {
	if m.isServiceMonitored(service) {
		return formatCPU(service.Process.CPU)
	}

	return ""
}

// getMem returns formatted memory usage for a service
func (m *Model) getMem(service *model.Service) string {
	if m.isServiceMonitored(service) {
		return formatMEM(float64(service.Process.Memory) / 1024 / 1024)
	}

	return ""
}

// getPID returns the process ID string for a running service
func (m *Model) getPID(service *model.Service) string {
	if m.isServiceMonitored(service) {
		return strconv.Itoa(service.Process.PID)
	}

	return ""
}

// isServiceMonitored returns true if service has valid monitoring data
func (m *Model) isServiceMonitored(service *model.Service) bool {
	return service.Status == model.StatusRunning && service.Process.PID != 0
}

// pad formats a number with leading zero
func pad(n int) string {
	return fmt.Sprintf("%02d", n)
}
