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

	m.ui.servicesKeys.ClearFilter.SetEnabled(m.state.filterQuery != "")

	panel := components.RenderPanel(components.PanelOptions{
		Title:   m.renderTitle(),
		Content: m.renderServices(),
		Status:  m.renderStatus(),
		Stats:   m.renderBottomLeft(),
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
	ready := m.getAllReadyServices()
	total := len(m.state.serviceIDs)

	phaseStr := string(m.state.phase)
	phaseStyle := m.theme.PhaseMutedStyle

	//nolint:exhaustive // stopped phase uses default styling
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
	return m.theme.PanelMutedStyle.Render("v" + config.Version)
}

// renderAppStats renders fuku's own CPU and memory usage with optional API indicator
func (m Model) renderAppStats() string {
	var parts []string

	if m.api != nil {
		if addr := m.api.Address(); addr != "" {
			dot := m.renderAPIDot()
			parts = append(parts, dot+" "+m.theme.PanelMutedStyle.Render(addr))
		}
	}

	if m.state.appCPU != 0 || m.state.appMEM != 0 {
		parts = append(parts, m.theme.PanelMutedStyle.Render(
			fmt.Sprintf("cpu %s • mem %s", formatCPU(m.state.appCPU), formatMEM(m.state.appMEM)),
		))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, m.theme.PanelMutedStyle.Render(" • "))
}

// renderAPIDot renders the colored dot indicator for API health
func (m Model) renderAPIDot() string {
	switch m.state.apiStatus {
	case apiStatusReady:
		return m.theme.APIDotConnected.Render(components.IndicatorDot)
	default:
		return m.theme.APIDotDisconnected.Render(components.IndicatorDot)
	}
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

	//nolint:perfsprint // readability over micro-optimization
	return m.theme.PanelMutedStyle.Render(fmt.Sprintf("profile • %s", m.state.profile))
}

// renderServices renders the services list or empty state
func (m Model) renderServices() string {
	if len(m.state.tiers) == 0 {
		return m.theme.EmptyStateStyle.Render("no services configured")
	}

	if m.isFiltering() && len(m.state.filteredIDs) == 0 {
		return m.theme.EmptyStateStyle.Render("no matching services")
	}

	return m.ui.servicesViewport.View()
}

// renderBottomLeft combines the filter bar and app stats for the bottom border
func (m Model) renderBottomLeft() string {
	filterBar := m.renderFilterBar()
	appStats := m.renderAppStats()

	switch {
	case filterBar != "" && appStats != "":
		return filterBar + m.theme.PanelMutedStyle.Render(" • ") + appStats
	case filterBar != "":
		return filterBar
	default:
		return appStats
	}
}

// renderFilterBar renders the filter input indicator
func (m Model) renderFilterBar() string {
	if !m.state.filterActive && m.state.filterQuery == "" {
		return ""
	}

	query := m.state.filterQuery

	maxLen := max(m.ui.width/3-4, 0)
	if maxLen > 0 {
		runes := []rune(query)
		if len(runes) > maxLen {
			query = string(runes[:maxLen])
		}
	}

	text := "/ " + query
	if m.state.filterActive {
		text += "_"
	}

	return m.theme.PanelMutedStyle.Render(text)
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
	timelineCol := strings.Repeat(" ", m.ui.layout.TimelineWidth+m.ui.layout.TimelineGapWidth)
	statusCol := fmt.Sprintf("%-*s", m.ui.layout.StatusWidth, "status")
	w := m.ui.layout.MetricWidth
	metricsCol := fmt.Sprintf("%*s%*s%*s%*s", w, "cpu", w, "mem", w, "pid", w, "uptime")

	header := nameCol + timelineCol + statusCol + metricsCol

	return m.theme.ServiceHeaderStyle.Width(m.getRowWidth()).Render(header)
}

// renderTier renders a tier header and its service rows
func (m Model) renderTier(tier Tier, currentIdx *int) string {
	rowWidth := m.getRowWidth()
	rows := make([]string, 0, len(tier.Services)+1)

	rows = append(rows, components.TierHeaderStyle.Width(rowWidth).Render(tier.Name))

	for _, serviceID := range tier.Services {
		service, exists := m.state.services[serviceID]
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

	if service.Status == StatusRunning && service.Watching {
		return m.getWatchIndicator(isSelected)
	}

	if service.Status != StatusStarting && service.Status != StatusStopping && service.Status != StatusRestarting {
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
		return components.IndicatorDot
	}

	return m.theme.IndicatorDotStyle.Render(components.IndicatorDot)
}

// renderTimeline renders the timeline strip for a service
func (m Model) renderTimeline(service *ServiceState, isSelected bool) string {
	tw := m.ui.layout.TimelineWidth
	if tw == 0 {
		return ""
	}

	if service.Timeline == nil {
		return strings.Repeat(" ", tw)
	}

	slots := service.Timeline.Slots()
	count := service.Timeline.Count()

	visibleObserved := min(tw, count)
	emptyPad := tw - visibleObserved
	observedStart := count - visibleObserved

	var b strings.Builder

	for i := observedStart; i < observedStart+visibleObserved; i++ {
		b.WriteString(m.timelineSlotStyle(slots[i], isSelected).Render(components.TimelineBlock))
	}

	emptyStyle := m.timelineSlotStyle(SlotEmpty, isSelected)
	for range emptyPad {
		b.WriteString(emptyStyle.Render(components.TimelineBlock))
	}

	return b.String()
}

// timelineSlotStyle returns the style for a timeline slot, composing with BgSelection when selected
func (m Model) timelineSlotStyle(slot TimelineSlot, isSelected bool) lipgloss.Style {
	if isSelected {
		switch slot {
		case SlotRunning:
			return m.theme.TimelineSelectedRunningStyle
		case SlotStarting:
			return m.theme.TimelineSelectedStartingStyle
		case SlotFailed:
			return m.theme.TimelineSelectedFailedStyle
		case SlotStopped:
			return m.theme.TimelineSelectedStoppedStyle
		default:
			return m.theme.TimelineSelectedEmptyStyle
		}
	}

	switch slot {
	case SlotRunning:
		return m.theme.TimelineRunningStyle
	case SlotStarting:
		return m.theme.TimelineStartingStyle
	case SlotFailed:
		return m.theme.TimelineFailedStyle
	case SlotStopped:
		return m.theme.TimelineStoppedStyle
	default:
		return m.theme.TimelineEmptyStyle
	}
}

// renderServiceRow renders a single service row with all columns
func (m Model) renderServiceRow(service *ServiceState, isSelected bool) string {
	rowWidth := m.getRowWidth()
	indicator := m.getServiceIndicator(service, isSelected)

	nameTextWidth := m.ui.layout.ServiceNameWidth - components.IndicatorColumnWidth
	name := components.TruncateAndPad(service.Name, nameTextWidth)
	nameCol := fmt.Sprintf("%s %s", indicator, name)

	timelineCol := m.renderTimeline(service, isSelected)

	statusCol := m.getStyledAndPaddedStatus(service, isSelected)
	details := m.getServiceDetails(service, isSelected)

	style := components.ServiceRowStyle
	if isSelected {
		style = m.theme.SelectedRowStyle
	}

	row := m.buildServiceRow(nameCol, timelineCol, statusCol, details, service.Error != nil, isSelected, rowWidth)

	return style.Width(rowWidth).Render(row)
}

// buildServiceRow positions details: errors left-aligned after status, metrics right-aligned to edge
func (m Model) buildServiceRow(nameCol, timelineCol, statusCol, details string, hasError, isSelected bool, rowWidth int) string {
	gap := strings.Repeat(" ", m.ui.layout.TimelineGapWidth)
	tail := gap + statusCol + details

	if hasError {
		remaining := max(rowWidth-lipgloss.Width(nameCol)-lipgloss.Width(timelineCol), 0)
		tail = components.PadRight(tail, remaining)
	}

	if isSelected && m.ui.layout.TimelineWidth > 0 {
		tail = lipgloss.NewStyle().Background(m.theme.BgSelection).Render(tail)
	}

	return nameCol + timelineCol + tail
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

	paddingLen := max(m.ui.layout.StatusWidth-len(statusStr), 0)

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
