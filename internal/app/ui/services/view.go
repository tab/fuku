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

	header := headerStyle.Render(m.renderHeader())
	panelHeight := m.ui.height - components.PanelHeightOffsetBottom

	var panel string
	if m.navigator.CurrentView() == navigation.ViewLogs {
		panel = noBorderTopPanelStyle.
			Width(m.ui.width - components.PanelBorderPadding).
			Height(panelHeight).
			Render(m.renderLogs())
	} else {
		panel = noBorderTopPanelStyle.
			Width(m.ui.width - components.PanelBorderPadding).
			Height(panelHeight).
			Render(m.renderServices())
	}

	help := helpWrapperStyle.Render(m.renderHelp())

	return lipgloss.JoinVertical(lipgloss.Left, header, panel, help)
}

func (m Model) renderInfo() string {
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

	info := fmt.Sprintf("%s  %d/%d ready",
		phaseStyle.Render(phaseStr),
		ready,
		total,
	)

	if m.navigator.CurrentView() == navigation.ViewLogs && m.logView.Autoscroll() {
		info += "  " + timestampStyle.Render("[autoscroll]")
	}

	return info
}

func (m Model) renderTitle() string {
	if m.loader.Active {
		return m.loader.Model.View() + " " + m.loader.Message()
	}

	if m.navigator.CurrentView() == navigation.ViewLogs {
		return headerTitleStyle.Render("logs")
	}

	return headerTitleStyle.Render("services")
}

func (m Model) renderHeader() string {
	title := m.renderTitle()
	info := m.renderInfo()

	panelWidth := m.ui.width - components.PanelBorderPadding
	infoWidth := lipgloss.Width(info)

	maxTitleWidth := panelWidth - infoWidth - components.HeaderSeparatorMin - components.HeaderFixedChars
	titleWidth := lipgloss.Width(title)

	if titleWidth > maxTitleWidth {
		title = truncate(title, maxTitleWidth)
		titleWidth = lipgloss.Width(title)
	}

	paddingWidth := panelWidth - titleWidth - infoWidth - components.HeaderFixedChars
	if paddingWidth < components.HeaderSeparatorMin {
		paddingWidth = components.HeaderSeparatorMin
	}

	prefix := borderCharStyle.Render("╭─ ")
	separator := borderCharStyle.Render(strings.Repeat("─", paddingWidth))
	suffix := borderCharStyle.Render(" ─╮")

	return prefix + title + "  " + separator + "  " + info + suffix
}

func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	if maxWidth == 1 {
		return "…"
	}

	if lipgloss.Width(s) <= maxWidth {
		return s
	}

	runes := []rune(s)
	for i := len(runes) - 1; i >= 0; i-- {
		truncated := string(runes[:i]) + "…"
		if lipgloss.Width(truncated) <= maxWidth {
			return truncated
		}
	}

	return "…"
}

func (m Model) renderServices() string {
	if len(m.state.tiers) == 0 {
		return emptyStateStyle.Render("No services configured")
	}

	tiers := make([]string, 0, len(m.state.tiers))

	currentIdx := 0
	maxNameLen := m.getMaxServiceNameLength()

	for _, tier := range m.state.tiers {
		tiers = append(tiers, m.renderTier(tier, &currentIdx, maxNameLen))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, tiers...)
	lines := strings.Split(content, "\n")

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

	return lipgloss.JoinVertical(lipgloss.Left, visibleLines...)
}

func (m Model) renderTier(tier Tier, currentIdx *int, maxNameLen int) string {
	rows := make([]string, 0, len(tier.Services)+1)

	rows = append(rows, tierHeaderStyle.Render(tier.Name))

	for _, serviceName := range tier.Services {
		service, exists := m.state.services[serviceName]
		if !exists {
			continue
		}

		isSelected := *currentIdx == m.state.selected
		rows = append(rows, m.renderServiceRow(service, isSelected, maxNameLen))

		*currentIdx++
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
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
