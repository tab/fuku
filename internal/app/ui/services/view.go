package services

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"fuku/internal/app/runtime"
	"fuku/internal/app/ui/components"
	"fuku/internal/app/ui/navigation"
)

// View renders the UI
func (m Model) View() string {
	if !m.state.ready {
		return "Initializing…"
	}

	var sections []string

	sections = append(sections, m.renderTitle())
	sections = append(sections, "")

	if m.navigator.CurrentView() == navigation.ViewLogs {
		logsPanel := activePanelStyle.
			Width(m.ui.width - components.PanelBorderPadding).
			Height(m.ui.height - components.PanelHeightOffset).
			Render(m.renderLogs())
		sections = append(sections, logsPanel)
	} else {
		servicesPanel := activePanelStyle.
			Width(m.ui.width - components.PanelBorderPadding).
			Height(m.ui.height - components.PanelHeightOffset).
			Render(m.renderServices())
		sections = append(sections, servicesPanel)
	}

	sections = append(sections, "")
	sections = append(sections, m.renderHelp())

	return strings.Join(sections, "\n")
}

func (m Model) renderTitle() string {
	titleText := ">_ services"

	if m.navigator.CurrentView() == navigation.ViewLogs {
		titleText = ">_ logs"
	}

	if m.loader.Active {
		titleText = m.loader.Model.View() + " " + m.loader.Message()
	}

	ready := m.getReadyServices()
	total := m.getTotalServices()

	phaseStr := string(m.state.phase)
	phaseStyle := phaseMutedStyle

	switch m.state.phase {
	case runtime.PhaseStartup:
		phaseStr = "Starting…"
		phaseStyle = phaseStartingStyle
	case runtime.PhaseRunning:
		phaseStr = "Running"
		phaseStyle = phaseRunningStyle
	case runtime.PhaseStopping:
		phaseStr = "Stopping"
		phaseStyle = phaseStoppingStyle
	}

	statusInfo := fmt.Sprintf("%s  %d/%d ready",
		phaseStyle.Render(phaseStr),
		ready,
		total,
	)

	if m.navigator.CurrentView() == navigation.ViewLogs && m.logView.Autoscroll() {
		statusInfo += "  " + timestampStyle.Render("[autoscroll]")
	}

	title := titleStyle.Render(titleText)
	info := statusStyle.Render(statusInfo)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		title,
		lipgloss.PlaceHorizontal(m.ui.width-lipgloss.Width(title)-components.PanelBorderPadding, lipgloss.Right, info),
	)
}

func (m Model) renderServices() string {
	if len(m.state.tiers) == 0 {
		return emptyStateStyle.Render("No services configured")
	}

	var content strings.Builder

	currentIdx := 0
	maxNameLen := m.getMaxServiceNameLength()

	for i, tier := range m.state.tiers {
		content.WriteString(m.renderTier(tier, &currentIdx, maxNameLen, i == 0))
	}

	contentStr := content.String()
	lines := strings.Split(contentStr, "\n")

	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	offset := m.ui.servicesViewport.YOffset
	if offset < 0 {
		offset = 0
	}

	maxOffset := len(lines) - m.ui.servicesViewport.Height
	if maxOffset < 0 {
		maxOffset = 0
	}

	if offset > maxOffset {
		offset = maxOffset
	}

	endLine := offset + m.ui.servicesViewport.Height
	if endLine > len(lines) {
		endLine = len(lines)
	}

	if offset >= len(lines) {
		return ""
	}

	visibleLines := lines[offset:endLine]

	return strings.Join(visibleLines, "\n")
}

func (m Model) renderTier(tier Tier, currentIdx *int, maxNameLen int, isFirst bool) string {
	var sb strings.Builder

	if !isFirst {
		sb.WriteString("\n")
	}

	tierHeader := tierHeaderStyle.Render(tier.Name)
	sb.WriteString(tierHeader + "\n")

	for _, serviceName := range tier.Services {
		service, exists := m.state.services[serviceName]
		if !exists {
			continue
		}

		isSelected := *currentIdx == m.state.selected
		sb.WriteString(m.renderServiceRow(service, isSelected, maxNameLen) + "\n")

		*currentIdx++
	}

	return sb.String()
}

func (m Model) renderServiceRow(service *ServiceState, isSelected bool, maxNameLen int) string {
	indicator := "  "
	if isSelected {
		indicator = "▸ "
	}

	if service.FSM != nil {
		state := service.FSM.Current()
		if state == Starting || state == Stopping || state == Restarting {
			indicator = "● "
		}
	}

	logCheckbox := "[ ]"
	if m.logView.IsEnabled(service.Name) {
		logCheckbox = "[x]"
	}

	uptime := m.getUptime(service)
	cpu := m.getCPU(service)
	mem := m.getMem(service)

	serviceName := service.Name

	availableWidth := m.ui.servicesViewport.Width - fixedColumnsWidth
	if availableWidth < minServiceNameWidth {
		availableWidth = minServiceNameWidth
	}

	if maxNameLen > availableWidth {
		maxNameLen = availableWidth
	}

	if len(serviceName) > maxNameLen {
		serviceName = serviceName[:maxNameLen-1] + "…"
	}

	row := fmt.Sprintf("%s%s %-*s  %-10s  %6s  %7s  %s",
		indicator,
		logCheckbox,
		maxNameLen,
		serviceName,
		string(service.Status),
		cpu,
		mem,
		uptime,
	)

	rowWidth := m.ui.servicesViewport.Width
	if rowWidth < 1 {
		rowWidth = m.ui.width - rowWidthPadding
	}

	if service.Error != nil {
		errorText := fmt.Sprintf(" (%s)", service.Error.Error())
		row += errorText
	}

	currentLen := len(row)
	if currentLen < rowWidth {
		row += strings.Repeat(" ", rowWidth-currentLen)
	}

	if isSelected {
		return selectedServiceRowStyle.Width(rowWidth).Render(row)
	}

	styledRow := m.applyRowStyles(row, service)

	return serviceRowStyle.Render(styledRow)
}

func (m Model) applyRowStyles(row string, service *ServiceState) string {
	result := row

	statusStr := string(service.Status)

	var styledStatus string

	switch service.Status {
	case StatusReady:
		styledStatus = statusReadyStyle.Render(statusStr)
	case StatusStarting:
		styledStatus = statusStartingStyle.Render(statusStr)
	case StatusFailed:
		styledStatus = statusFailedStyle.Render(statusStr)
	case StatusStopped:
		styledStatus = statusStoppedStyle.Render(statusStr)
	default:
		styledStatus = statusStr
	}

	result = strings.Replace(result, statusStr, styledStatus, 1)

	if service.Error != nil {
		errorText := fmt.Sprintf("(%s)", service.Error.Error())
		styledError := errorStyle.Render(errorText)
		result = strings.Replace(result, errorText, styledError, 1)
	}

	return result
}

func (m Model) renderLogs() string {
	return m.logView.View()
}

func (m Model) renderHelp() string {
	var helpView string

	if m.navigator.CurrentView() == navigation.ViewLogs {
		helpView = m.ui.help.View(LogsHelpKeyMap(m.ui.keys))
	} else {
		helpView = m.ui.help.View(ServicesHelpKeyMap(m.ui.keys))
	}

	return helpStyle.Render(helpView)
}
