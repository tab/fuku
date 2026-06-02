package services

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"fuku/internal/app/ui/components"
	"fuku/internal/config"
)

const (
	asideTitleIndent  = " "
	asideRowIndent    = "  "
	asideLabelGap     = 2
	asideDottedChar   = "┄"
	asideTabSeparator = " • "
)

// AsideTab identifies a tab in the aside panel
type AsideTab string

// AsideTab values
const (
	AsideTabConfig AsideTab = "config"
	AsideTabEnv    AsideTab = "env"
	AsideTabHealth AsideTab = "health"
)

// asideTabs lists the tabs in the order they appear in the bar
var asideTabs = []AsideTab{
	AsideTabConfig,
	AsideTabEnv,
	AsideTabHealth,
}

// asideTabIndex returns the position of tab in asideTabs, or -1 when tab is not a known value
func asideTabIndex(tab AsideTab) int {
	for i, t := range asideTabs {
		if t == tab {
			return i
		}
	}

	return -1
}

// nextAsideTab cycles forward through tabs, returning the first tab when the input is unknown
func nextAsideTab(tab AsideTab) AsideTab {
	idx := asideTabIndex(tab)
	if idx < 0 {
		return asideTabs[0]
	}

	return asideTabs[(idx+1)%len(asideTabs)]
}

// prevAsideTab cycles backward through tabs, returning the first tab when the input is unknown
func prevAsideTab(tab AsideTab) AsideTab {
	idx := asideTabIndex(tab)
	if idx < 0 {
		return asideTabs[0]
	}

	return asideTabs[(idx-1+len(asideTabs))%len(asideTabs)]
}

// cardRow is a label-value pair rendered inside a section, with optional value style
type cardRow struct {
	label string
	value string
	style lipgloss.Style
}

// renderAsideLines returns the aside panel as a slice of lines so callers can join them with the services panel by direct concatenation
func (m Model) renderAsideLines(width, height int) []string {
	innerWidth := max(width-components.PanelInnerPadding, 1)

	borderStyle := m.asidePanelBorderStyle()
	border := func(s string) string { return borderStyle.Render(s) }

	contentHeight := max(height-components.PanelBorderHeight, 1)

	topBorder := components.BuildTopBorder(border, m.asideBorderTabs(), m.asideScrollIndicator(), innerWidth)
	bottomBorder := components.BuildBottomBorder(border, "", m.renderVersion(), innerWidth)

	contentLines, prePadded := m.asideVisibleLinesPadded(innerWidth, contentHeight)

	lines := make([]string, 0, contentHeight+2)
	lines = append(lines, topBorder)

	if prePadded {
		lines = components.AppendPrePaddedContentLines(lines, contentLines, border)
	} else {
		lines = components.AppendContentLines(lines, contentLines, innerWidth, border)
	}

	lines = append(lines, bottomBorder)

	return lines
}

// asideVisibleLinesPadded slices the pre-padded aside lines based on the viewport's YOffset; returns the lines along with a flag indicating whether they are already padded to innerWidth (allowing the caller to skip per-line width measurement). Falls back to a viewport.View() round-trip when the precomputed cache is empty (e.g., in tests that drive renderAside directly).
func (m Model) asideVisibleLinesPadded(innerWidth, contentHeight int) ([]string, bool) {
	if len(m.ui.asideLines) == 0 {
		return components.SplitAndPadContent(m.ui.asideViewport.View(), contentHeight), false
	}

	total := len(m.ui.asideLines)
	yoff := min(m.ui.asideViewport.YOffset(), total)
	end := min(yoff+contentHeight, total)

	out := make([]string, contentHeight)
	copy(out, m.ui.asideLines[yoff:end])

	if end-yoff < contentHeight {
		emptyPad := strings.Repeat(" ", innerWidth)
		for i := end - yoff; i < contentHeight; i++ {
			out[i] = emptyPad
		}
	}

	return out, true
}

// updateAsideContent refreshes the aside viewport contents from the currently selected service and active tab; debounced to once per wall-clock second so per-tick redraws skip the full content rebuild
func (m *Model) updateAsideContent() {
	if !m.state.asideOpen {
		return
	}

	width := m.ui.asideViewport.Width()
	if width <= 0 {
		return
	}

	service := m.getSelectedService()
	key := m.asideContentCacheKey(service, width)

	if m.ui.asideCache != nil && m.ui.asideCache.key == key {
		return
	}

	body := m.asideContent(service, width)

	if m.ui.asideCache != nil {
		m.ui.asideCache.key = key
		m.ui.asideCache.content = body
	}

	m.ui.asideViewport.SetContent(body)
	m.ui.asideLines = padAsideLines(strings.Split(body, "\n"), width)
}

// padAsideLines right-pads each line with spaces up to width so the render path can skip per-line width calculation; honors ANSI-aware width measurement
func padAsideLines(lines []string, width int) []string {
	out := make([]string, len(lines))

	for i, line := range lines {
		w := lipgloss.Width(line)
		if w >= width {
			out[i] = line

			continue
		}

		out[i] = line + strings.Repeat(" ", width-w)
	}

	return out
}

// asideContentCacheKey returns a stable string that captures every input affecting asideContent's output
func (m Model) asideContentCacheKey(service *ServiceState, innerWidth int) string {
	if service == nil {
		return strconv.Itoa(innerWidth) + "|" + string(m.state.asideTab) + "|nil"
	}

	errStr := ""
	if service.Error != nil {
		errStr = service.Error.Error()
	}

	return strconv.Itoa(innerWidth) + "|" + string(m.state.asideTab) + "|" + service.ID + "|" + string(service.Status) + "|" + strconv.Itoa(service.PID) + "|" + errStr + "|" + service.LifecycleAt.String() + "|" + m.state.now.Truncate(time.Second).String()
}

// asideScrollIndicator returns a small percent string when the aside content exceeds the viewport, otherwise empty
func (m Model) asideScrollIndicator() string {
	if m.ui.asideViewport.AtTop() && m.ui.asideViewport.AtBottom() {
		return ""
	}

	percent := m.asideScrollPercent()

	return m.theme.PanelMutedStyle.Render(strconv.Itoa(percent) + "%")
}

// asideScrollPercent returns the current scroll position as a 0-100 integer; bucketed via int truncation so consecutive single-line scrolls of long content do not invalidate the panel cache on every keypress
func (m Model) asideScrollPercent() int {
	return int(m.ui.asideViewport.ScrollPercent() * 100)
}

// asideBorderTabs renders the tab labels as a single styled string for the panel border title area
func (m Model) asideBorderTabs() string {
	separator := m.theme.PanelMutedStyle.Render(asideTabSeparator)
	activeIdx := asideTabIndex(m.state.asideTab)

	parts := make([]string, 0, len(asideTabs))

	for i, tab := range asideTabs {
		label := string(tab)

		if i == activeIdx {
			parts = append(parts, m.theme.StatusRunningStyle.Render(label))

			continue
		}

		parts = append(parts, m.theme.PanelMutedStyle.Render(label))
	}

	return strings.Join(parts, separator)
}

// asideContent builds the body content for a service (tab content; tabs live in the border)
func (m Model) asideContent(service *ServiceState, innerWidth int) string {
	if service == nil {
		return components.ContentTopMarginStyle.Render(m.theme.PlaceholderStyle.Render("no service selected"))
	}

	entry := m.lookupServiceConfig(service.Name)

	return components.ContentTopMarginStyle.Render(m.asideTabContent(service, entry, innerWidth))
}

// asideTabContent dispatches rendering based on the active tab
func (m Model) asideTabContent(service *ServiceState, entry *config.Service, innerWidth int) string {
	switch m.state.asideTab {
	case AsideTabHealth:
		return m.asideHealthTab(service, entry, innerWidth)
	case AsideTabEnv:
		return m.asideEnvTab(service, innerWidth)
	default:
		return m.asideConfigTab(service, entry, innerWidth)
	}
}

// asideConfigTab renders the config tab content (meta, readiness, logs, watch cards)
func (m Model) asideConfigTab(service *ServiceState, entry *config.Service, innerWidth int) string {
	parts := make([]string, 0, 6)

	if errorCard := m.asideErrorCard(service, innerWidth); errorCard != "" {
		parts = append(parts, errorCard)
	}

	if metaCard := m.asideMetaCard(service, entry, innerWidth); metaCard != "" {
		parts = append(parts, metaCard)
	}

	if readinessCard := m.asideReadinessCard(entry, innerWidth); readinessCard != "" {
		parts = append(parts, readinessCard)
	}

	if logsCard := m.asideLogsCard(entry, innerWidth); logsCard != "" {
		parts = append(parts, logsCard)
	}

	if watchCard := m.asideWatchCard(entry, innerWidth); watchCard != "" {
		parts = append(parts, watchCard)
	}

	return strings.Join(parts, "\n\n")
}

// asideHealthTab renders the health tab content with probe, process, retry, and status cards stacked vertically
func (m Model) asideHealthTab(service *ServiceState, entry *config.Service, innerWidth int) string {
	cards := make([]string, 0, 4)

	if probe := m.asideProbeCard(entry, innerWidth); probe != "" {
		cards = append(cards, probe)
	}

	if proc := m.asideProcessCard(service, innerWidth); proc != "" {
		cards = append(cards, proc)
	}

	if retry := m.asideRetryCard(innerWidth); retry != "" {
		cards = append(cards, retry)
	}

	if status := m.asideStatusCard(service, innerWidth); status != "" {
		cards = append(cards, status)
	}

	return strings.Join(cards, "\n\n")
}

// asideProbeCard renders the readiness probe configuration as a meta-styled card
func (m Model) asideProbeCard(entry *config.Service, innerWidth int) string {
	if entry == nil || entry.Readiness == nil || entry.Readiness.Type == "" {
		return ""
	}

	r := entry.Readiness
	rows := m.readinessRows(r)

	if r.Interval > 0 {
		rows = append(rows, cardRow{label: "interval", value: r.Interval.String()})
	}

	if r.Timeout > 0 {
		rows = append(rows, cardRow{label: "timeout", value: r.Timeout.String()})
	}

	return m.asideSection("probe", rows, innerWidth)
}

// readinessRows returns the base rows shared by every readiness card: the type row plus the endpoint row chosen by URL > Address > Pattern precedence (the three are mutually exclusive across the supported probe types)
func (m Model) readinessRows(r *config.Readiness) []cardRow {
	rows := []cardRow{
		{label: "type", value: r.Type, style: m.theme.StatusRunningStyle},
	}

	switch {
	case r.URL != "":
		rows = append(rows, cardRow{label: "url", value: r.URL})
	case r.Address != "":
		rows = append(rows, cardRow{label: "address", value: r.Address})
	case r.Pattern != "":
		rows = append(rows, cardRow{label: "pattern", value: r.Pattern})
	}

	return rows
}

// asideProcessCard renders the running process id and uptime
func (m Model) asideProcessCard(service *ServiceState, innerWidth int) string {
	if service.PID == 0 {
		return ""
	}

	rows := []cardRow{
		{label: "pid", value: strconv.Itoa(service.PID), style: m.theme.StatusRunningStyle},
	}

	if uptime := m.getUptime(service); uptime != "" {
		rows = append(rows, cardRow{label: "uptime", value: uptime})
	}

	return m.asideSection("process", rows, innerWidth)
}

// asideRetryCard renders the global retry policy
func (m Model) asideRetryCard(innerWidth int) string {
	if m.cfg == nil {
		return ""
	}

	attempts := m.cfg.Retry.Attempts
	backoff := m.cfg.Retry.Backoff

	if attempts == 0 && backoff == 0 {
		return ""
	}

	rows := make([]cardRow, 0, 2)

	if attempts > 0 {
		rows = append(rows, cardRow{label: "attempts", value: strconv.Itoa(attempts), style: m.theme.PhaseStartingStyle})
	}

	if backoff > 0 {
		rows = append(rows, cardRow{label: "backoff", value: backoff.String()})
	}

	return m.asideSection("retry", rows, innerWidth)
}

// asideStatusCard renders the current lifecycle state and how long it has held
func (m Model) asideStatusCard(service *ServiceState, innerWidth int) string {
	rows := []cardRow{
		{label: "state", value: string(service.Status), style: m.asideStatusStyle(service.Status)},
	}

	if !service.LifecycleAt.IsZero() && !m.state.now.IsZero() {
		rows = append(rows, cardRow{label: "duration", value: formatElapsed(m.state.now.Sub(service.LifecycleAt))})
	}

	return m.asideSection("status", rows, innerWidth)
}

// formatElapsed renders a duration as HH:MM:SS (or MM:SS when under an hour)
func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return pad(hours) + ":" + pad(minutes) + ":" + pad(seconds)
	}

	return pad(minutes) + ":" + pad(seconds)
}

// asideEnvTab renders the env tab content: merged contents of the service's .env files in load order
func (m Model) asideEnvTab(service *ServiceState, innerWidth int) string {
	card := m.asideDotenvCard(service, innerWidth)
	if card == "" {
		return m.theme.PlaceholderStyle.Render("no environment variables available")
	}

	return card
}

// asideDotenvCard renders the merged contents of the service's .env files under an "environment" section title with dotted separators between rows
func (m Model) asideDotenvCard(service *ServiceState, innerWidth int) string {
	if m.dotenv == nil {
		return ""
	}

	merged := m.dotenv.Env(service.ID)
	if len(merged) == 0 {
		return ""
	}

	rows := make([]cardRow, 0, len(merged))
	for _, kv := range merged {
		rows = append(rows, cardRow{label: kv.Key, value: kv.Value})
	}

	return m.asideDottedSection("environment", rows, innerWidth)
}

// lookupServiceConfig safely returns the config entry for a service name
func (m Model) lookupServiceConfig(name string) *config.Service {
	if m.cfg == nil || m.cfg.Services == nil {
		return nil
	}

	return m.cfg.Services[name]
}

// asideStatusStyle returns the style used for the status badge
func (m Model) asideStatusStyle(status Status) lipgloss.Style {
	switch status {
	case StatusRunning:
		return m.theme.StatusRunningStyle
	case StatusStarting, StatusRestarting, StatusStopping:
		return m.theme.StatusStartingStyle
	case StatusFailed:
		return m.theme.StatusFailedStyle
	case StatusStopped:
		return m.theme.StatusStoppedStyle
	default:
		return m.theme.PanelMutedStyle
	}
}

// asideMetaCard renders the tier, dir, and command rows in a card
func (m Model) asideMetaCard(service *ServiceState, entry *config.Service, innerWidth int) string {
	rows := make([]cardRow, 0, 3)

	if service.Tier != "" {
		rows = append(rows, cardRow{label: "tier", value: service.Tier})
	}

	if entry != nil && entry.Dir != "" {
		rows = append(rows, cardRow{label: "dir", value: entry.Dir})
	}

	if entry != nil {
		rows = append(rows, cardRow{label: "command", value: resolveServiceCommand(entry.Command)})
	}

	if len(rows) == 0 {
		return ""
	}

	return m.asideSection("meta", rows, innerWidth)
}

// resolveServiceCommand returns command or the default when command is empty
func resolveServiceCommand(command string) string {
	if command == "" {
		return config.DefaultServiceCommand
	}

	return command
}

// asideErrorCard renders the friendly error reason when service.Error is set
func (m Model) asideErrorCard(service *ServiceState, innerWidth int) string {
	if service.Error == nil {
		return ""
	}

	rows := []cardRow{
		{label: "reason", value: renderError(service.Error), style: m.theme.StatusFailedStyle},
	}

	return m.asideSection("error", rows, innerWidth)
}

// asideReadinessCard renders the readiness check details when configured
func (m Model) asideReadinessCard(entry *config.Service, innerWidth int) string {
	if entry == nil || entry.Readiness == nil || entry.Readiness.Type == "" {
		return ""
	}

	return m.asideSection("readiness", m.readinessRows(entry.Readiness), innerWidth)
}

// asideLogsCard renders configured per-service log outputs in a card
func (m Model) asideLogsCard(entry *config.Service, innerWidth int) string {
	if entry == nil || entry.Logs == nil || len(entry.Logs.Output) == 0 {
		return ""
	}

	rows := []cardRow{
		{label: "output", value: strings.Join(entry.Logs.Output, ", ")},
	}

	return m.asideSection("logs", rows, innerWidth)
}

// asideWatchCard renders watch configuration in a card
func (m Model) asideWatchCard(entry *config.Service, innerWidth int) string {
	if entry == nil || entry.Watch == nil {
		return ""
	}

	w := entry.Watch
	if len(w.Include) == 0 && len(w.Ignore) == 0 && len(w.Shared) == 0 && w.Debounce == 0 {
		return ""
	}

	rows := make([]cardRow, 0, 4)

	if len(w.Include) > 0 {
		rows = append(rows, cardRow{label: "include", value: strings.Join(w.Include, ", ")})
	}

	if len(w.Ignore) > 0 {
		rows = append(rows, cardRow{label: "ignore", value: strings.Join(w.Ignore, ", ")})
	}

	if len(w.Shared) > 0 {
		rows = append(rows, cardRow{label: "shared", value: strings.Join(w.Shared, ", ")})
	}

	if w.Debounce > 0 {
		rows = append(rows, cardRow{label: "debounce", value: w.Debounce.String(), style: m.theme.PhaseStartingStyle})
	}

	return m.asideSection("watch", rows, innerWidth)
}

// asideSection renders a labeled key-value section: optional title line followed by indented label-value rows; returns empty when there are no rows or no usable width
func (m Model) asideSection(title string, rows []cardRow, innerWidth int) string {
	return m.renderAsideSection(title, rows, innerWidth, false)
}

// asideDottedSection is asideSection with a dotted separator line inserted between consecutive rows; used when rows are dense enough to benefit from row-level dividers
func (m Model) asideDottedSection(title string, rows []cardRow, innerWidth int) string {
	return m.renderAsideSection(title, rows, innerWidth, true)
}

func (m Model) renderAsideSection(title string, rows []cardRow, innerWidth int, dottedSeparators bool) string {
	if len(rows) == 0 {
		return ""
	}

	rowAvailable := innerWidth - len(asideRowIndent)
	if rowAvailable < 1 {
		return ""
	}

	labelWidth := computeLabelWidth(rows)

	capacity := len(rows) + 1
	if dottedSeparators {
		capacity = len(rows)*2 + 1
	}

	lines := make([]string, 0, capacity)

	if title != "" {
		lines = append(lines, asideTitleIndent+m.theme.PanelMutedStyle.Bold(true).Render(title))
	}

	var separator string
	if dottedSeparators {
		separator = asideRowIndent + m.theme.PanelMutedStyle.Render(strings.Repeat(asideDottedChar, rowAvailable))
	}

	for i, row := range rows {
		if dottedSeparators && i > 0 {
			lines = append(lines, separator)
		}

		lines = append(lines, asideRowIndent+m.asideRow(row, labelWidth, rowAvailable))
	}

	return strings.Join(lines, "\n")
}

// asideRow formats a single label-value row aligned to labelWidth and truncated to available
func (m Model) asideRow(row cardRow, labelWidth, available int) string {
	labelStyled := m.theme.PanelMutedStyle.Render(row.label)
	labelDisplayWidth := lipgloss.Width(labelStyled)

	pad := max(labelWidth-labelDisplayWidth, 1)
	labelBlock := labelStyled + strings.Repeat(" ", pad)

	remaining := available - lipgloss.Width(labelBlock)
	if remaining < 1 {
		return labelStyled
	}

	value := row.value
	if lipgloss.Width(value) > remaining {
		value = truncateText(value, remaining)
	}

	return labelBlock + row.style.Render(value)
}

// computeLabelWidth returns the widest label across rows plus a small gap to leave space before the value column
func computeLabelWidth(rows []cardRow) int {
	longest := 0

	for _, row := range rows {
		if w := lipgloss.Width(row.label); w > longest {
			longest = w
		}
	}

	return longest + asideLabelGap
}

// truncateText shortens text to fit within width using a trailing ellipsis
func truncateText(s string, width int) string {
	if width <= 0 {
		return ""
	}

	if lipgloss.Width(s) <= width {
		return s
	}

	if width == 1 {
		return "…"
	}

	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i-1]) + "…"
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
	}

	return "…"
}
