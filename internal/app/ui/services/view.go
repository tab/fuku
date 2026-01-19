package services

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"fuku/internal/app/runtime"
	"fuku/internal/app/ui/components"
)

// View renders the UI
func (m Model) View() string {
	if !m.state.ready {
		return "Initializing…"
	}

	header := m.renderHeader()
	panelHeight := m.ui.height - components.PanelHeightPadding

	panel := lipgloss.NewStyle().
		Width(m.ui.width - components.PanelBorderPadding).
		Height(panelHeight).
		Render(m.renderServices())

	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, panel, footer)
}

func (m Model) renderInfo() string {
	ready := m.getReadyServices()
	total := m.getTotalServices()

	phaseStr := string(m.state.phase)
	phaseStyle := components.PhaseMutedStyle

	switch m.state.phase {
	case runtime.PhaseStartup:
		phaseStr = "Starting…"
		phaseStyle = components.PhaseStartingStyle
	case runtime.PhaseRunning:
		phaseStr = "Running"
		phaseStyle = components.PhaseRunningStyle
	case runtime.PhaseStopping:
		phaseStr = "Stopping"
		phaseStyle = components.PhaseStoppingStyle
	}

	return fmt.Sprintf("%s  %d/%d ready",
		phaseStyle.Render(phaseStr),
		ready,
		total,
	)
}

func (m Model) renderTitle() string {
	if m.loader.Active {
		return m.loader.Model.View() + " " + m.loader.Message()
	}

	return components.HeaderTitleStyle.Render("services")
}

func (m Model) renderHeader() string {
	title := m.renderTitle()
	info := m.renderInfo()

	return components.RenderHeader(m.ui.width, title, info)
}

func (m Model) renderServices() string {
	if len(m.state.tiers) == 0 {
		return components.EmptyStateStyle.Render("No services configured")
	}

	return m.ui.servicesViewport.View()
}

func (m Model) renderColumnHeaders(maxNameLen int) string {
	availableWidth := m.ui.servicesViewport.Width - components.FixedColumnsWidth
	if availableWidth < components.ServiceNameMinWidth {
		availableWidth = components.ServiceNameMinWidth
	}

	if maxNameLen > availableWidth {
		maxNameLen = availableWidth
	}

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

	return components.ServiceHeaderStyle.Render(header)
}

func (m Model) renderTier(tier Tier, currentIdx *int, maxNameLen int) string {
	rows := make([]string, 0, len(tier.Services)+1)

	rows = append(rows, components.TierHeaderStyle.Render(tier.Name))

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

func (m Model) getServiceIndicator(service *ServiceState, isSelected bool) string {
	defaultIndicator := " "
	if isSelected {
		defaultIndicator = components.Current
	}

	if service.FSM == nil {
		return defaultIndicator
	}

	state := service.FSM.Current()
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

func (m Model) renderServiceRow(service *ServiceState, isSelected bool, maxNameLen int) string {
	indicator := m.getServiceIndicator(service, isSelected)

	uptime := m.getUptime(service)
	cpu := m.getCPU(service)
	mem := m.getMem(service)
	pid := m.getPID(service)

	availableWidth := m.ui.servicesViewport.Width - components.FixedColumnsWidth
	if availableWidth < components.ServiceNameMinWidth {
		availableWidth = components.ServiceNameMinWidth
	}

	if maxNameLen > availableWidth {
		maxNameLen = availableWidth
	}

	serviceName := components.Truncate(service.Name, maxNameLen)
	paddedServiceName := padServiceName(serviceName, maxNameLen)

	row := fmt.Sprintf(
		"%s %s  %-*s  %*s  %*s  %*s  %*s",
		indicator,
		paddedServiceName,
		components.ColWidthStatus, string(service.Status),
		components.ColWidthCPU, cpu,
		components.ColWidthMem, mem,
		components.ColWidthPID, pid,
		components.ColWidthUptime, uptime,
	)

	rowWidth := m.ui.servicesViewport.Width
	if rowWidth < 1 {
		rowWidth = m.ui.width - components.RowWidthPadding
	}

	if service.Error != nil {
		rowDisplayWidth := lipgloss.Width(row)
		availableWidth := rowWidth - rowDisplayWidth

		errorText := truncateErrorMessage(simplifyErrorMessage(service.Error), availableWidth)
		row += errorText
	}

	currentDisplayWidth := lipgloss.Width(row)
	if currentDisplayWidth < rowWidth {
		row += strings.Repeat(" ", rowWidth-currentDisplayWidth)
	}

	if isSelected {
		return components.SelectedServiceRowStyle.Width(rowWidth).Render(row)
	}

	styledRow := m.applyRowStyles(row, service)

	return components.ServiceRowStyle.Render(styledRow)
}

func (m Model) applyRowStyles(row string, service *ServiceState) string {
	result := row

	statusStr := string(service.Status)

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

	result = strings.Replace(result, statusStr, styledStatus, 1)

	if service.Error != nil {
		errorText := fmt.Sprintf("(%s)", simplifyErrorMessage(service.Error))
		styledError := components.ErrorStyle.Render(errorText)
		result = strings.Replace(result, errorText, styledError, 1)
	}

	return result
}

func (m Model) renderFooter() string {
	return components.RenderFooter(m.ui.width, m.ui.help.View(m.ui.servicesKeys))
}
