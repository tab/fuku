package services

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"fuku/internal/app/runtime"
)

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "Initializing…"
	}

	if m.quitting {
		return ""
	}

	var sections []string

	sections = append(sections, m.renderTitle())
	sections = append(sections, "")

	if m.viewMode == ViewModeLogs {
		logsPanel := activePanelStyle.
			Width(m.width - 2).
			Height(m.height - 10).
			Render(m.renderLogs())
		sections = append(sections, logsPanel)
	} else {
		servicesPanel := activePanelStyle.
			Width(m.width - 2).
			Height(m.height - 10).
			Render(m.renderServices())
		sections = append(sections, servicesPanel)
	}

	sections = append(sections, "")
	sections = append(sections, m.renderHelp())

	return strings.Join(sections, "\n")
}

func (m Model) renderTitle() string {
	titleText := ">_ services"

	if m.viewMode == ViewModeLogs {
		titleText = ">_ logs"
	}

	if m.loader.Active {
		titleText = m.loader.Model.View() + " " + m.loader.Message()
	}

	ready := m.getReadyServices()
	total := m.getTotalServices()

	phaseStr := string(m.phase)
	phaseStyle := phaseMutedStyle

	switch m.phase {
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

	title := titleStyle.Render(titleText)
	info := statusStyle.Render(statusInfo)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		title,
		lipgloss.PlaceHorizontal(m.width-lipgloss.Width(title)-2, lipgloss.Right, info),
	)
}

func (m Model) renderServices() string {
	if len(m.tiers) == 0 {
		return emptyStateStyle.Render("No services configured")
	}

	var content strings.Builder

	currentIdx := 0
	maxNameLen := m.getMaxServiceNameLength()

	for i, tier := range m.tiers {
		content.WriteString(m.renderTier(tier, &currentIdx, maxNameLen, i == 0))
	}

	contentStr := content.String()
	lines := strings.Split(contentStr, "\n")

	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	offset := m.servicesViewport.YOffset
	if offset < 0 {
		offset = 0
	}

	maxOffset := len(lines) - m.servicesViewport.Height
	if maxOffset < 0 {
		maxOffset = 0
	}

	if offset > maxOffset {
		offset = maxOffset
	}

	endLine := offset + m.servicesViewport.Height
	if endLine > len(lines) {
		endLine = len(lines)
	}

	if offset >= len(lines) {
		return ""
	}

	visibleLines := lines[offset:endLine]

	return strings.Join(visibleLines, "\n")
}

func (m Model) renderTier(tier TierView, currentIdx *int, maxNameLen int, isFirst bool) string {
	var sb strings.Builder

	if !isFirst {
		sb.WriteString("\n")
	}

	tierHeader := tierHeaderStyle.Render(tier.Name)
	sb.WriteString(tierHeader + "\n")

	for _, serviceName := range tier.Services {
		service, exists := m.services[serviceName]
		if !exists {
			continue
		}

		isSelected := *currentIdx == m.selected
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
	if service.LogEnabled {
		logCheckbox = "[x]"
	}

	statusRaw := string(service.Status)
	uptimeRaw := m.getUptimeRaw(service)
	cpuRaw := m.getCPURaw(service)
	memRaw := m.getMemRaw(service)

	serviceName := service.Name

	availableWidth := m.servicesViewport.Width - 45
	if availableWidth < 20 {
		availableWidth = 20
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
		statusRaw,
		cpuRaw,
		memRaw,
		uptimeRaw,
	)

	rowWidth := m.servicesViewport.Width
	if rowWidth < 1 {
		rowWidth = m.width - 8
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
	if len(strings.TrimSpace(m.logsViewport.View())) == 0 {
		return emptyStateStyle.Render("No logs enabled. Press 'space' to toggle service logs. Press 'l' to return to services view.")
	}

	return m.logsViewport.View()
}

func (m Model) renderHelp() string {
	helpView := m.help.View(m.keys)
	return helpStyle.Render(helpView)
}
