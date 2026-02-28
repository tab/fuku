package services

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"fuku/internal/app/bus"
	"fuku/internal/app/ui/components"
	"fuku/internal/config"
)

// View renders the UI
func (m Model) View() tea.View {
	if !m.state.ready {
		return tea.NewView("initializing…")
	}

	panelWidth := m.ui.width
	panelHeight := m.ui.height - components.PanelHeightPadding

	panel := components.RenderPanel(components.PanelOptions{
		Title:   m.renderTitle(),
		Content: m.renderServices(),
		Status:  m.renderStatus(),
		Stats:   m.renderAppStats(),
		Version: m.renderVersion(),
		Help:    m.renderHelp(),
		Tips:    m.renderTip(),
		Height:  panelHeight,
		Width:   panelWidth,
	})

	v := tea.NewView(components.AppContainerStyle.Render(panel))
	v.AltScreen = true

	return v
}

// renderStatus renders the status bar with phase and service counts
func (m Model) renderStatus() string {
	ready := m.getReadyServices()
	total := m.getTotalServices()

	phaseStr := string(m.state.phase)
	phaseStyle := m.theme.PhaseMutedStyle

	switch m.state.phase {
	case bus.PhaseStartup:
		phaseStr = "starting…"
		phaseStyle = m.theme.PhaseStartingStyle
	case bus.PhaseRunning:
		phaseStr = "running"
		phaseStyle = m.theme.PhaseRunningStyle
	case bus.PhaseStopping:
		phaseStr = "stopping"
		phaseStyle = m.theme.PhaseStoppingStyle
	}

	return fmt.Sprintf("%s %d/%d ready",
		phaseStyle.Render(phaseStr),
		ready,
		total,
	)
}

// renderVersion renders the version string
func (m Model) renderVersion() string {
	return m.theme.PanelMutedStyle.Render(fmt.Sprintf("v%s", config.Version))
}

// renderAppStats renders fuku's own CPU and memory usage
func (m Model) renderAppStats() string {
	if m.state.appCPU == 0 && m.state.appMEM == 0 {
		return ""
	}

	return m.theme.PanelMutedStyle.Render(
		fmt.Sprintf("cpu %s • mem %s", formatCPU(m.state.appCPU), formatMEM(m.state.appMEM)),
	)
}

// renderHelp renders the help text with keybindings
func (m Model) renderHelp() string {
	return m.theme.HelpStyle.Render(m.ui.help.View(m.ui.servicesKeys))
}

// renderTip returns the current rotating tip or empty string if tips disabled
func (m Model) renderTip() string {
	if !m.ui.showTips {
		return ""
	}

	rotation := m.ui.tickCounter / components.TipRotationTicks
	tipIndex := (m.ui.tipOffset + rotation) % len(components.Tips)

	return components.Tips[tipIndex].Render(m.theme)
}

// renderTitle renders the title with optional loading spinner
func (m Model) renderTitle() string {
	if m.loader.Active {
		var b strings.Builder
		b.WriteString(m.loader.Model.View())
		b.WriteString(components.LoaderSpacerStyle.Render(m.loader.Message()))

		return b.String()
	}

	return "services"
}

// renderServices renders the services list or empty state
func (m Model) renderServices() string {
	if len(m.state.tiers) == 0 {
		return m.theme.EmptyStateStyle.Render("no services configured")
	}

	return m.ui.servicesViewport.View()
}

// getRowWidth returns the available width for service rows
func (m Model) getRowWidth() int {
	rowWidth := m.ui.servicesViewport.Width()
	if rowWidth < 1 {
		rowWidth = m.ui.width - components.RowWidthPadding
	}

	return rowWidth
}

// renderColumnHeaders renders the column headers row
func (m Model) renderColumnHeaders() string {
	nameCol := strings.Repeat(" ", m.ui.layout.ServiceNameWidth)
	statusCol := fmt.Sprintf("%-*s", m.ui.layout.StatusWidth, "status")
	w := m.ui.layout.MetricWidth
	metricsCol := fmt.Sprintf("%*s%*s%*s%*s", w, "cpu", w, "mem", w, "pid", w, "uptime")

	header := nameCol + statusCol + metricsCol

	return m.theme.ServiceHeaderStyle.Width(m.getRowWidth()).Render(header)
}

// renderTier renders a tier header and its service rows
func (m Model) renderTier(tier Tier, currentIdx *int) string {
	rowWidth := m.getRowWidth()
	rows := make([]string, 0, len(tier.Services)+1)

	rows = append(rows, components.TierHeaderStyle.Width(rowWidth).Render(tier.Name))

	for _, serviceName := range tier.Services {
		service, exists := m.state.services[serviceName]
		if !exists {
			continue
		}

		isSelected := *currentIdx == m.state.selected
		rows = append(rows, m.renderServiceRow(service, isSelected))

		*currentIdx++
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	return components.TierContainerStyle.Render(content)
}

// getServiceIndicator returns the selection or status indicator for a service
func (m Model) getServiceIndicator(service *ServiceState, isSelected bool) string {
	defaultIndicator := components.IndicatorEmpty
	if isSelected {
		defaultIndicator = components.IndicatorSelected
	}

	if service.FSM == nil {
		if service.Watching && service.Status == StatusRunning {
			return m.getWatchIndicator(isSelected)
		}

		return defaultIndicator
	}

	state := service.FSM.Current()

	if state == Running && service.Watching {
		return m.getWatchIndicator(isSelected)
	}

	if state != Starting && state != Stopping && state != Restarting {
		return defaultIndicator
	}

	if service.Blink == nil {
		return defaultIndicator
	}

	if isSelected {
		return service.Blink.Frame()
	}

	return service.Blink.Render(m.theme.IndicatorActiveStyle)
}

// getWatchIndicator returns the styled watch indicator
func (m Model) getWatchIndicator(isSelected bool) string {
	if isSelected {
		return components.IndicatorWatch
	}

	return m.theme.IndicatorWatchStyle.Render(components.IndicatorWatch)
}

// renderServiceRow renders a single service row with all columns
func (m Model) renderServiceRow(service *ServiceState, isSelected bool) string {
	rowWidth := m.getRowWidth()
	indicator := m.getServiceIndicator(service, isSelected)

	nameTextWidth := m.ui.layout.ServiceNameWidth - components.IndicatorColumnWidth
	name := components.TruncateAndPad(service.Name, nameTextWidth)
	nameCol := fmt.Sprintf("%s %s", indicator, name)

	statusCol := m.getStyledAndPaddedStatus(service, isSelected)
	details := m.getServiceDetails(service, isSelected)

	style := components.ServiceRowStyle
	if isSelected {
		style = m.theme.SelectedRowStyle
	}

	row := m.buildServiceRow(nameCol, statusCol, details, service.Error != nil, rowWidth)

	return style.Width(rowWidth).Render(row)
}

// buildServiceRow positions details: errors left-aligned after status, metrics right-aligned to edge
func (m Model) buildServiceRow(nameCol, statusCol, details string, hasError bool, rowWidth int) string {
	row := nameCol + statusCol + details
	if hasError {
		return components.PadRight(row, rowWidth)
	}

	return row
}

// getServiceDetails returns either error message or metrics columns
func (m Model) getServiceDetails(service *ServiceState, isSelected bool) string {
	if service.Error != nil {
		errorMsg := fmt.Sprintf("%s%s", components.ErrorPadding, renderError(service.Error))
		if !isSelected {
			return m.theme.ErrorStyle.Render(errorMsg)
		}

		return errorMsg
	}

	return fmt.Sprintf("%*s%*s%*s%*s",
		m.ui.layout.MetricWidth, m.getCPU(service),
		m.ui.layout.MetricWidth, m.getMem(service),
		m.ui.layout.MetricWidth, m.getPID(service),
		m.ui.layout.MetricWidth, m.getUptime(service),
	)
}

// getStyledAndPaddedStatus returns the styled status string with padding
func (m Model) getStyledAndPaddedStatus(service *ServiceState, isSelected bool) string {
	statusStr := string(service.Status)

	paddingLen := m.ui.layout.StatusWidth - len(statusStr)
	if paddingLen < 0 {
		paddingLen = 0
	}

	padding := strings.Repeat(components.IndicatorEmpty, paddingLen)

	if isSelected {
		return statusStr + padding
	}

	var styledStatus string

	switch service.Status {
	case StatusRunning:
		styledStatus = m.theme.StatusRunningStyle.Render(statusStr)
	case StatusStarting:
		styledStatus = m.theme.StatusStartingStyle.Render(statusStr)
	case StatusFailed:
		styledStatus = m.theme.StatusFailedStyle.Render(statusStr)
	case StatusStopped:
		styledStatus = m.theme.StatusStoppedStyle.Render(statusStr)
	default:
		styledStatus = statusStr
	}

	return styledStatus + padding
}
