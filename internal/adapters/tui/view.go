package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

// View renders the UI under the registry's read lock
func (m Model) View() tea.View {
	if !m.state.ready {
		return tea.NewView("initializing…")
	}

	var view tea.View

	m.registry.Read(func(snapshot *model.Snapshot) {
		m.snapshot = snapshot
		view = m.render()
	})

	return view
}

// render builds the UI from the bound read model
func (m Model) render() tea.View {
	mainWidth, asideWidth := m.panelWidths()
	panelHeight := m.ui.height - terminal.PanelHeightPadding

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

	footer := terminal.RenderFooter(m.renderHelp(asideShown), m.renderTip(), m.ui.width)
	content := lipgloss.JoinVertical(lipgloss.Left, row, footer)

	v := tea.NewView(terminal.AppContainerStyle.Render(content))
	v.AltScreen = true

	return v
}

// helpKeyMap returns a copy of the services key map with the aside and clear-filter bindings toggled for the state
func (m Model) helpKeyMap(asideShown bool) KeyMap {
	km := m.ui.servicesKeys
	km.AsideClose.SetEnabled(asideShown)
	km.AsideTabNext.SetEnabled(asideShown)
	km.AsideTabPrev.SetEnabled(asideShown)
	km.ClearFilter.SetEnabled(!asideShown && m.state.filterQuery != "")

	return km
}

// renderServicesPanelLines returns the services panel as lines, cached while its cheap-to-hash inputs are unchanged
func (m Model) renderServicesPanelLines(mainWidth, panelHeight int, asideShown bool, panelVersion string) []string {
	key := m.servicesPanelCacheKey(mainWidth, panelHeight, asideShown, panelVersion)

	if m.ui.servicesPanelCache.key == key {
		return m.ui.servicesPanelCache.lines
	}

	lines := terminal.RenderPanelLines(terminal.PanelOptions{
		Title:       m.renderTitle(),
		Content:     m.renderServices(),
		Status:      m.renderStatus(),
		Stats:       m.renderBottomLeft(),
		Version:     panelVersion,
		Height:      panelHeight,
		Width:       mainWidth,
		BorderStyle: m.servicesPanelBorderStyle(),
	})

	m.ui.servicesPanelCache.key = key
	m.ui.servicesPanelCache.lines = lines

	return lines
}

// servicesPanelCacheKey concatenates every input that affects the services panel render into a stable key
func (m Model) servicesPanelCacheKey(mainWidth, panelHeight int, asideShown bool, panelVersion string) string {
	loaderTick := 0
	if m.loader.Active {
		loaderTick = m.ui.tickCounter
	}

	return strconv.Itoa(mainWidth) + "|" +
		strconv.Itoa(panelHeight) + "|" +
		strconv.Itoa(m.ui.servicesViewport.YOffset()) + "|" +
		strconv.FormatUint(m.ui.servicesContentVersion, 10) + "|" +
		strconv.Itoa(loaderTick) + "|" +
		strconv.FormatBool(m.state.asideFocused) + "|" +
		strconv.FormatBool(asideShown) + "|" +
		string(m.snapshot.Phase) + "|" +
		m.snapshot.API.Address + "|" +
		strconv.FormatBool(m.snapshot.API.Listening) + "|" +
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

	phaseStr := string(m.snapshot.Phase)
	phaseStyle := m.theme.PhaseMutedStyle

	//nolint:exhaustive // stopped phase uses default styling
	switch m.snapshot.Phase {
	case "", model.PhaseStartup:
		phaseStr = "starting…"
		phaseStyle = m.theme.PhaseStartingStyle
	case model.PhaseRunning:
		phaseStyle = m.theme.PhaseRunningStyle
	case model.PhaseStopping:
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
	current := m.theme.CurrentVersionStyle.Render("v" + buildinfo.Version)
	if m.state.availableVersion == "" {
		return current
	}

	return current + m.theme.PanelMutedStyle.Render(" - ") + m.theme.LatestVersionStyle.Render("↑ "+m.state.availableVersion)
}

// renderAppStats renders fuku's own CPU and memory usage with optional API indicator
func (m Model) renderAppStats() string {
	var parts []string

	if !m.asideVisible() && m.snapshot.API.Address != "" {
		parts = append(parts, m.renderAPIDot()+" "+m.theme.PanelMutedStyle.Render(m.snapshot.API.Address))
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

// renderAPIDot renders the colored dot indicator for the API listening state
func (m Model) renderAPIDot() string {
	if m.snapshot.API.Listening {
		return m.theme.APIDotConnected.Render(terminal.IndicatorDot)
	}

	return m.theme.APIDotDisconnected.Render(terminal.IndicatorDot)
}

// renderHelp renders the help text with keybindings; bindings vary by aside state
func (m Model) renderHelp(asideShown bool) string {
	keys := m.helpKeyMap(asideShown)

	if m.state.asideOpen {
		return m.theme.HelpStyle.Render(m.ui.help.View(newAsideHelpKeyMap(keys)))
	}

	return m.theme.HelpStyle.Render(m.ui.help.View(keys))
}

// renderTip returns the current rotating tip or empty string if tips disabled
func (m Model) renderTip() string {
	if !m.ui.showTips {
		return ""
	}

	rotation := m.ui.tickCounter / terminal.UITipRotationTicks
	tipIndex := (m.ui.tipOffset + rotation) % len(terminal.Tips)

	return terminal.Tips[tipIndex].Render(m.theme)
}

// renderTitle renders the title with optional loading spinner
func (m Model) renderTitle() string {
	if m.loader.Active {
		var b strings.Builder
		b.WriteString(m.loader.Model.View())
		b.WriteString(terminal.LoaderSpacerStyle.Render(m.loader.Message()))

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
	if len(m.snapshot.Tiers) == 0 {
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
	runes := []rune(query)

	if maxLen > 0 && len(runes) > maxLen {
		query = string(runes[:maxLen])
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
		rowWidth = m.ui.width - terminal.RowWidthPadding
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
