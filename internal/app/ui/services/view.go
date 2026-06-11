package services

import (
	"fmt"
	"strconv"
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

	mainWidth, asideWidth := m.panelWidths()
	panelHeight := m.ui.height - components.PanelHeightPadding

	asideShown := asideWidth > 0

	panelVersion := m.renderVersion()
	if asideShown {
		panelVersion = ""
	}

	panelLines := m.renderServicesPanelLines(mainWidth, panelHeight, asideShown, panelVersion)

	rowLines := panelLines

	if asideWidth > 0 {
		asideLines := m.renderAsideLines(asideWidth, panelHeight)
		rowLines = make([]string, len(panelLines))

		for i := range panelLines {
			rowLines[i] = panelLines[i] + asideLines[i]
		}
	}

	row := strings.Join(rowLines, "\n")

	footer := components.RenderFooter(m.renderHelp(asideShown), m.renderTip(), m.ui.width)
	content := lipgloss.JoinVertical(lipgloss.Left, row, footer)

	v := tea.NewView(components.AppContainerStyle.Render(content))
	v.AltScreen = true

	return v
}

// helpKeyMap returns a copy of the services key map with aside and clear-filter bindings toggled per current state, without mutating m.ui.servicesKeys
func (m Model) helpKeyMap(asideShown bool) KeyMap {
	km := m.ui.servicesKeys
	km.AsideClose.SetEnabled(asideShown)
	km.AsideTabNext.SetEnabled(asideShown)
	km.AsideTabPrev.SetEnabled(asideShown)
	km.ClearFilter.SetEnabled(!asideShown && m.state.filterQuery != "")

	return km
}

// renderServicesPanelLines returns the services panel as a slice of lines, using a cached value when none of the cheap-to-hash inputs have changed
func (m Model) renderServicesPanelLines(mainWidth, panelHeight int, asideShown bool, panelVersion string) []string {
	key := m.servicesPanelCacheKey(mainWidth, panelHeight, asideShown, panelVersion)

	if m.ui.servicesPanelCache != nil && m.ui.servicesPanelCache.key == key {
		return m.ui.servicesPanelCache.lines
	}

	lines := components.RenderPanelLines(components.PanelOptions{
		Title:       m.renderTitle(),
		Content:     m.renderServices(),
		Status:      m.renderStatus(),
		Stats:       m.renderBottomLeft(),
		Version:     panelVersion,
		Height:      panelHeight,
		Width:       mainWidth,
		BorderStyle: m.servicesPanelBorderStyle(),
	})

	if m.ui.servicesPanelCache != nil {
		m.ui.servicesPanelCache.key = key
		m.ui.servicesPanelCache.lines = lines
	}

	return lines
}

// servicesPanelCacheKey concatenates every input that affects the services panel render into a stable key
func (m Model) servicesPanelCacheKey(mainWidth, panelHeight int, asideShown bool, panelVersion string) string {
	loaderTick := 0
	if m.loader != nil && m.loader.Active {
		loaderTick = m.ui.tickCounter
	}

	return strconv.Itoa(mainWidth) + "|" +
		strconv.Itoa(panelHeight) + "|" +
		strconv.Itoa(m.ui.servicesViewport.YOffset()) + "|" +
		strconv.FormatUint(m.ui.servicesContentVersion, 10) + "|" +
		strconv.Itoa(loaderTick) + "|" +
		strconv.FormatBool(m.state.asideFocused) + "|" +
		strconv.FormatBool(asideShown) + "|" +
		string(m.state.phase) + "|" +
		strconv.Itoa(int(m.state.apiStatus)) + "|" +
		strconv.Itoa(int(m.state.appCPU*100)) + "|" +
		strconv.Itoa(int(m.state.appMEM*100)) + "|" +
		strconv.FormatBool(m.state.filterActive) + "|" +
		m.state.filterQuery + "|" +
		m.state.profile + "|" +
		m.state.availableVersion + "|" +
		panelVersion
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
		phaseStyle = m.theme.PhaseRunningStyle
	case bus.PhaseStopping:
		phaseStyle = m.theme.PhaseStoppingStyle
	}

	return fmt.Sprintf("%s %d/%d ready",
		phaseStyle.Render(phaseStr),
		ready,
		total,
	)
}

// renderVersion renders the version string with an optional update-available hint
func (m Model) renderVersion() string {
	current := m.theme.CurrentVersionStyle.Render("v" + config.Version)
	if m.state.availableVersion == "" {
		return current
	}

	return current + m.theme.PanelMutedStyle.Render(" - ") + m.theme.LatestVersionStyle.Render("↑ "+m.state.availableVersion)
}

// renderAppStats renders fuku's own CPU and memory usage with optional API indicator
func (m Model) renderAppStats() string {
	var parts []string

	if !m.asideVisible() && m.api != nil {
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

// renderHelp renders the help text with keybindings; bindings vary by aside state
func (m Model) renderHelp(asideShown bool) string {
	keys := m.helpKeyMap(asideShown)

	if m.state.asideOpen {
		return m.theme.HelpStyle.Render(m.ui.help.View(NewAsideHelpKeyMap(keys)))
	}

	return m.theme.HelpStyle.Render(m.ui.help.View(keys))
}

// renderTip returns the current rotating tip or empty string if tips disabled
func (m Model) renderTip() string {
	if !m.ui.showTips {
		return ""
	}

	rotation := m.ui.tickCounter / components.UITipRotationTicks
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

	if m.asideVisible() {
		return m.theme.PanelMutedStyle.Render(m.state.profile)
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

	mainWidth, _ := m.panelWidths()

	maxLen := max(mainWidth/3-4, 0)
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
	if m.asideVisible() {
		return ""
	}

	nameCol := strings.Repeat(" ", m.ui.layout.ServiceNameWidth)
	leftFlex := strings.Repeat(" ", m.ui.layout.LeftFlexWidth)
	timelineCol := strings.Repeat(" ", m.ui.layout.TimelineWidth+m.ui.layout.TimelineGapWidth)
	statusCol := fmt.Sprintf("%-*s", m.ui.layout.StatusWidth, "status")
	rightFlex := strings.Repeat(" ", m.ui.layout.RightFlexWidth)
	metricsCol := m.renderMetricHeaders()

	header := nameCol + leftFlex + timelineCol + statusCol + rightFlex + metricsCol

	return m.theme.ServiceHeaderStyle.Width(m.getRowWidth()).Render(header)
}

// renderMetricHeaders renders the metric column headers based on the active layout
func (m Model) renderMetricHeaders() string {
	w := m.ui.layout.MetricWidth
	if w <= 0 || m.ui.layout.MetricColumns <= 0 {
		return ""
	}

	labels := metricLabels(m.ui.layout.MetricColumns)

	var b strings.Builder

	for _, label := range labels {
		fmt.Fprintf(&b, "%*s", w, label)
	}

	return b.String()
}

// metricLabels returns the column labels for the given metric column count
func metricLabels(metricColumns int) []string {
	all := []string{"cpu", "mem", "pid", "uptime"}
	if metricColumns >= len(all) {
		return all
	}

	return all[:metricColumns]
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

	blocks := m.timelineSlotBlocks(isSelected)

	var b strings.Builder

	for i := observedStart; i < observedStart+visibleObserved; i++ {
		b.WriteString(blocks[slots[i]])
	}

	emptyBlock := blocks[SlotEmpty]
	for range emptyPad {
		b.WriteString(emptyBlock)
	}

	return b.String()
}

// timelineSlotBlocks pre-renders the styled timeline block character once per (selection state) so the inner loop avoids repeated style.Render calls
func (m Model) timelineSlotBlocks(isSelected bool) [5]string {
	return [5]string{
		SlotEmpty:    m.timelineSlotStyle(SlotEmpty, isSelected).Render(components.TimelineBlock),
		SlotRunning:  m.timelineSlotStyle(SlotRunning, isSelected).Render(components.TimelineBlock),
		SlotStarting: m.timelineSlotStyle(SlotStarting, isSelected).Render(components.TimelineBlock),
		SlotFailed:   m.timelineSlotStyle(SlotFailed, isSelected).Render(components.TimelineBlock),
		SlotStopped:  m.timelineSlotStyle(SlotStopped, isSelected).Render(components.TimelineBlock),
	}
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

	nameTextWidth := max(m.ui.layout.ServiceNameWidth-components.IndicatorColumnWidth-components.ServiceNameTrailingGap, 1)
	name := components.TruncateAndPad(service.Name, nameTextWidth)
	nameCol := fmt.Sprintf("%s %s ", indicator, name)

	style := components.ServiceRowStyle
	if isSelected {
		style = m.theme.SelectedRowStyle
	}

	if m.asideVisible() {
		statusText := m.styledStatus(service, isSelected)
		gap := max(m.ui.layout.ContentWidth-lipgloss.Width(nameCol)-lipgloss.Width(statusText), 0)

		return style.Width(rowWidth).Render(nameCol + strings.Repeat(" ", gap) + statusText)
	}

	timelineCol := m.renderTimeline(service, isSelected)
	statusCol := m.getStyledAndPaddedStatus(service, isSelected)
	details := m.getServiceDetails(service, isSelected)

	row := m.buildServiceRow(rowParts{
		name:       nameCol,
		timeline:   timelineCol,
		status:     statusCol,
		details:    details,
		hasError:   service.Error != nil,
		isSelected: isSelected,
	}, rowWidth)

	return style.Width(rowWidth).Render(row)
}

// rowParts groups the column segments for a service row
type rowParts struct {
	name       string
	timeline   string
	status     string
	details    string
	hasError   bool
	isSelected bool
}

// buildServiceRow positions sections: name left, timeline+status center, metrics right
func (m Model) buildServiceRow(parts rowParts, rowWidth int) string {
	leftFlex := strings.Repeat(" ", m.ui.layout.LeftFlexWidth)
	timelineGap := strings.Repeat(" ", m.ui.layout.TimelineGapWidth)
	rightFlex := strings.Repeat(" ", m.ui.layout.RightFlexWidth)
	tail := timelineGap + parts.status + rightFlex + parts.details

	if parts.hasError {
		remaining := max(rowWidth-lipgloss.Width(parts.name)-lipgloss.Width(leftFlex)-lipgloss.Width(parts.timeline), 0)
		tail = components.PadRight(timelineGap+parts.status+parts.details, remaining)
	}

	if parts.isSelected && m.ui.layout.TimelineWidth > 0 {
		tail = m.theme.SelectionBgStyle.Render(tail)
	}

	return parts.name + leftFlex + parts.timeline + tail
}

// getServiceDetails returns either error message or metrics columns
func (m Model) getServiceDetails(service *ServiceState, isSelected bool) string {
	if service.Error != nil {
		errorMsg := fmt.Sprintf("%s%s", components.RowErrorPadding, renderError(service.Error))
		if !isSelected {
			return m.theme.ErrorStyle.Render(errorMsg)
		}

		return errorMsg
	}

	w := m.ui.layout.MetricWidth
	if w <= 0 || m.ui.layout.MetricColumns <= 0 {
		return ""
	}

	values := []string{
		m.getCPU(service),
		m.getMem(service),
		m.getPID(service),
		m.getUptime(service),
	}

	count := min(m.ui.layout.MetricColumns, len(values))

	var b strings.Builder

	for i := range count {
		fmt.Fprintf(&b, "%*s", w, fitMetric(values[i], w))
	}

	return b.String()
}

// fitMetric truncates a metric value with an ellipsis when it would exceed the column width
func fitMetric(s string, w int) string {
	if w <= 0 {
		return ""
	}

	if lipgloss.Width(s) <= w {
		return s
	}

	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i])
		if lipgloss.Width(candidate) <= w-1 {
			return candidate + "…"
		}
	}

	return "…"
}

// styledStatus returns the status string with the lifecycle-appropriate color (no padding)
func (m Model) styledStatus(service *ServiceState, isSelected bool) string {
	statusStr := string(service.Status)
	if isSelected {
		return statusStr
	}

	switch service.Status {
	case StatusPending:
		return m.theme.StatusPendingStyle.Render(statusStr)
	case StatusRunning:
		return m.theme.StatusRunningStyle.Render(statusStr)
	case StatusStarting:
		return m.theme.StatusStartingStyle.Render(statusStr)
	case StatusFailed:
		return m.theme.StatusFailedStyle.Render(statusStr)
	case StatusStopped:
		return m.theme.StatusStoppedStyle.Render(statusStr)
	default:
		return statusStr
	}
}

// getStyledAndPaddedStatus returns the styled status string padded to fit StatusWidth
func (m Model) getStyledAndPaddedStatus(service *ServiceState, isSelected bool) string {
	statusStr := string(service.Status)
	paddingLen := max(m.ui.layout.StatusWidth-len(statusStr), 0)
	padding := strings.Repeat(components.IndicatorEmpty, paddingLen)

	return m.styledStatus(service, isSelected) + padding
}
