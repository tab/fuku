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
	phaseColor := ColorMuted

	switch m.phase {
	case runtime.PhaseStartup:
		phaseStr = "Starting…"
		phaseColor = ColorStarting
	case runtime.PhaseRunning:
		phaseStr = "Running"
		phaseColor = ColorReady
	case runtime.PhaseStopping:
		phaseStr = "Stopping"
		phaseColor = ColorFailed
	}

	statusInfo := fmt.Sprintf("%s  %d/%d ready",
		lipgloss.NewStyle().Foreground(phaseColor).Render(phaseStr),
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
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(2).
			Render("No services configured")
	}

	var content strings.Builder

	currentIdx := 0

	for _, tier := range m.tiers {
		content.WriteString(m.renderTier(tier, &currentIdx))
	}

	m.servicesViewport.SetContent(content.String())

	return m.servicesViewport.View()
}

func (m Model) renderTier(tier TierView, currentIdx *int) string {
	tierHeader := tierHeaderStyle.Render(tier.Name)

	var sb strings.Builder
	sb.WriteString(tierHeader + "\n")

	for _, serviceName := range tier.Services {
		service, exists := m.services[serviceName]
		if !exists {
			continue
		}

		isSelected := *currentIdx == m.selected
		sb.WriteString(m.renderServiceRow(service, isSelected) + "\n")

		*currentIdx++
	}

	sb.WriteString("\n")

	return sb.String()
}

func (m Model) renderServiceRow(service *ServiceState, isSelected bool) string {
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

	row := fmt.Sprintf("%s%s %-20s  %-10s  %6s  %7s  %s",
		indicator,
		logCheckbox,
		service.Name,
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
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(2).
			Render("No logs enabled. Press 'space' to toggle service logs. Press 'l' to return to services view.")
	}

	return m.logsViewport.View()
}

func (m Model) renderHelp() string {
	helpView := m.help.View(m.keys)
	return helpStyle.Render(helpView)
}
