package services

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"fuku/internal/app/bus"
	"fuku/internal/app/ui/components"
	"fuku/internal/config"
)

// View renders the UI
func (m Model) View() string {
	if !m.state.ready {
		return "Initializing…"
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

	return components.AppContainerStyle.Render(panel)
}

// renderStatus renders the status bar with phase and service counts
func (m Model) renderStatus() string {
	ready := m.getReadyServices()
	total := m.getTotalServices()

	phaseStr := string(m.state.phase)
	phaseStyle := components.PhaseMutedStyle

	switch m.state.phase {
	case bus.PhaseStartup:
		phaseStr = "Starting…"
		phaseStyle = components.PhaseStartingStyle
	case bus.PhaseRunning:
		phaseStr = "Running"
		phaseStyle = components.PhaseRunningStyle
	case bus.PhaseStopping:
		phaseStr = "Stopping"
		phaseStyle = components.PhaseStoppingStyle
	}

	return fmt.Sprintf("%s %d/%d ready",
		phaseStyle.Render(phaseStr),
		ready,
		total,
	)
}

// renderVersion renders the version string
func (m Model) renderVersion() string {
	return fmt.Sprintf("v%s", config.Version)
}

// renderAppStats renders fuku's own CPU and memory usage
func (m Model) renderAppStats() string {
	if m.state.appCPU == 0 && m.state.appMEM == 0 {
		return ""
	}

	return fmt.Sprintf("cpu %s • mem %s", formatCPU(m.state.appCPU), formatMEM(m.state.appMEM))
}

// renderHelp renders the help text with keybindings
func (m Model) renderHelp() string {
	return components.HelpStyle.Render(m.ui.help.View(m.ui.servicesKeys))
}

// renderTip returns the current rotating tip or empty string if tips disabled
func (m Model) renderTip() string {
	if !m.ui.showTips {
		return ""
	}

	rotation := m.ui.tickCounter / components.TipRotationTicks
	tipIndex := (m.ui.tipOffset + rotation) % len(components.Tips)

	return components.Tips[tipIndex]
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
		return components.EmptyStateStyle.Render("No services configured")
	}

	return m.ui.servicesViewport.View()
}

// getRowWidth returns the available width for service rows
func (m Model) getRowWidth() int {
	rowWidth := m.ui.servicesViewport.Width
	if rowWidth < 1 {
		rowWidth = m.ui.width - components.RowWidthPadding
	}

	return rowWidth
}

// clampNameWidth constrains service name width to available space
func (m Model) clampNameWidth(maxNameLen int) int {
	availableWidth := m.getRowWidth() - components.FixedColumnsWidth
	if availableWidth < components.ServiceNameMinWidth {
		availableWidth = components.ServiceNameMinWidth
	}

	if maxNameLen > availableWidth {
		return availableWidth
	}

	return maxNameLen
}

// renderColumnHeaders renders the column headers row
func (m Model) renderColumnHeaders(maxNameLen int) string {
	rowWidth := m.getRowWidth()
	maxNameLen = m.clampNameWidth(maxNameLen)
	prefixWidth := components.ColWidthIndicator + 1 + maxNameLen

	header := fmt.Sprintf(
		"%*s  %-*s  %*s  %*s  %*s  %*s",
		prefixWidth, "",
		components.ColWidthStatus, "status",
		components.ColWidthCPU, "cpu",
		components.ColWidthMem, "mem",
		components.ColWidthPID, "pid",
		components.ColWidthUptime, "uptime",
	)

	return components.ServiceHeaderStyle.Width(rowWidth).Render(header)
}

// renderTier renders a tier header and its service rows
func (m Model) renderTier(tier Tier, currentIdx *int, maxNameLen int) string {
	rowWidth := m.getRowWidth()
	rows := make([]string, 0, len(tier.Services)+1)

	rows = append(rows, components.TierHeaderStyle.Width(rowWidth).Render(tier.Name))

	for _, serviceName := range tier.Services {
		service, exists := m.state.services[serviceName]
		if !exists {
			continue
		}

		isSelected := *currentIdx == m.state.selected
		rows = append(rows, m.renderServiceRow(service, isSelected, maxNameLen))

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

	return service.Blink.Render(components.IndicatorActiveStyle)
}

// getWatchIndicator returns the styled watch indicator
func (m Model) getWatchIndicator(isSelected bool) string {
	if isSelected {
		return components.IndicatorWatch
	}

	return components.IndicatorWatchStyle.Render(components.IndicatorWatch)
}

// renderServiceRow renders a single service row with all columns
func (m Model) renderServiceRow(service *ServiceState, isSelected bool, maxNameLen int) string {
	rowWidth := m.getRowWidth()
	maxNameLen = m.clampNameWidth(maxNameLen)
	indicator := m.getServiceIndicator(service, isSelected)
	serviceName := components.TruncateAndPad(service.Name, maxNameLen)
	status := m.getStyledAndPaddedStatus(service, isSelected)

	var b strings.Builder

	fmt.Fprintf(&b, "%s %s  %s  %*s  %*s  %*s  %*s",
		indicator,
		serviceName,
		status,
		components.ColWidthCPU, m.getCPU(service),
		components.ColWidthMem, m.getMem(service),
		components.ColWidthPID, m.getPID(service),
		components.ColWidthUptime, m.getUptime(service),
	)

	if service.Error != nil {
		errorAvailWidth := rowWidth - lipgloss.Width(b.String())
		errorMsg := truncateErrorMessage(renderError(service.Error), errorAvailWidth)

		if !isSelected {
			errorMsg = components.ErrorStyle.Render(errorMsg)
		}

		b.WriteString(errorMsg)
	}

	row := components.PadRight(b.String(), rowWidth)

	if isSelected {
		return components.SelectedServiceRowStyle.Width(rowWidth).Render(row)
	}

	return components.ServiceRowStyle.Width(rowWidth).Render(row)
}

// getStyledAndPaddedStatus returns the styled status string with padding
func (m Model) getStyledAndPaddedStatus(service *ServiceState, isSelected bool) string {
	statusStr := string(service.Status)

	paddingLen := components.ColWidthStatus - len(statusStr)
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
		styledStatus = components.StatusRunningStyle.Render(statusStr)
	case StatusStarting:
		styledStatus = components.StatusStartingStyle.Render(statusStr)
	case StatusFailed:
		styledStatus = components.StatusFailedStyle.Render(statusStr)
	case StatusStopped:
		styledStatus = components.StatusStoppedStyle.Render(statusStr)
	default:
		styledStatus = statusStr
	}

	return styledStatus + padding
}
