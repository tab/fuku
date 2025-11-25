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

	var panel string
	if m.navigator.IsLogs() {
		panel = m.renderLogs()
	} else {
		panel = lipgloss.NewStyle().
			Width(m.ui.width - components.PanelBorderPadding).
			Height(panelHeight).
			Render(m.renderServices())
	}

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

	info := fmt.Sprintf("%s  %d/%d ready",
		phaseStyle.Render(phaseStr),
		ready,
		total,
	)

	if m.navigator.IsLogs() && m.logView.Autoscroll() {
		info += "  " + components.TimestampStyle.Render("[autoscroll]")
	}

	return info
}

func (m Model) renderTitle() string {
	if m.loader.Active {
		return m.loader.Model.View() + " " + m.loader.Message()
	}

	if m.navigator.IsLogs() {
		return components.HeaderTitleStyle.Render("logs")
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
	defaultIndicator := "  "
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
		return service.Blink.Frame() + " "
	}

	return service.Blink.Render(components.IndicatorActiveStyle) + " "
}

func (m Model) renderServiceRow(service *ServiceState, isSelected bool, maxNameLen int) string {
	indicator := m.getServiceIndicator(service, isSelected)

	logCheckbox := components.Empty
	if m.logView.IsEnabled(service.Name) {
		logCheckbox = components.Selected
	}

	uptime := m.getUptime(service)
	cpu := m.getCPU(service)
	mem := m.getMem(service)

	availableWidth := m.ui.servicesViewport.Width - components.FixedColumnsWidth
	if availableWidth < components.ServiceNameMinWidth {
		availableWidth = components.ServiceNameMinWidth
	}

	if maxNameLen > availableWidth {
		maxNameLen = availableWidth
	}

	serviceName := components.Truncate(service.Name, maxNameLen)
	paddedServiceName := padServiceName(serviceName, maxNameLen)

	row := fmt.Sprintf("%s%s %s  %-10s  %6s  %7s  %s",
		indicator,
		logCheckbox,
		paddedServiceName,
		string(service.Status),
		cpu,
		mem,
		uptime,
	)

	rowWidth := m.ui.servicesViewport.Width
	if rowWidth < 1 {
		rowWidth = m.ui.width - components.RowWidthPadding
	}

	if service.Error != nil {
		rowDisplayWidth := lipgloss.Width(row)
		availableWidth := rowWidth - rowDisplayWidth

		errorText := truncateErrorMessage(service.Error.Error(), availableWidth)
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
		errorText := fmt.Sprintf("(%s)", service.Error.Error())
		styledError := components.ErrorStyle.Render(errorText)
		result = strings.Replace(result, errorText, styledError, 1)
	}

	return result
}

func (m Model) renderLogs() string {
	return m.logView.View()
}

func (m Model) renderFooter() string {
	var helpText string

	if m.navigator.IsLogs() {
		helpText = m.ui.help.View(m.ui.logsKeys)
	} else {
		helpText = m.ui.help.View(m.ui.servicesKeys)
	}

	return components.RenderFooter(m.ui.width, helpText)
}
