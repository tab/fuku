package tui

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
)

const (
	asideTitleIndent  = " "
	asideRowIndent    = "  "
	asideLabelGap     = 2
	asideDottedChar   = "┄"
	asideTabSeparator = " • "
	asidePlaceholder  = "—"
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

// renderAsideLines returns the aside panel as lines, so callers concatenate it with the services panel
func (m Model) renderAsideLines(width, height int) []string {
	innerWidth := max(width-terminal.PanelInnerPadding, 1)

	borderStyle := m.asidePanelBorderStyle()
	border := func(s string) string { return borderStyle.Render(s) }

	contentHeight := max(height-terminal.PanelBorderHeight, 1)

	topBorder := terminal.BuildTopBorder(border, m.asideBorderTabs(), m.asideScrollIndicator(), innerWidth)
	bottomBorder := terminal.BuildBottomBorder(border, "", m.renderVersion(), innerWidth)

	contentLines, prePadded := m.asideVisibleLinesPadded(innerWidth, contentHeight)

	lines := make([]string, 0, contentHeight+2)
	lines = append(lines, topBorder)

	if prePadded {
		lines = terminal.AppendPrePaddedContentLines(lines, contentLines, border)
	} else {
		lines = terminal.AppendContentLines(lines, contentLines, innerWidth, border)
	}

	lines = append(lines, bottomBorder)

	return lines
}

// asideVisibleLinesPadded returns the visible aside lines and whether they came padded from the cache
func (m Model) asideVisibleLinesPadded(innerWidth, contentHeight int) ([]string, bool) {
	if len(m.ui.asideLines) == 0 {
		return terminal.SplitAndPadContent(m.ui.asideViewport.View(), contentHeight), false
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

// updateAsideContent rebuilds the aside viewport for the selected service and tab, at most once per wall-clock second
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

	if m.ui.asideCache.key == key {
		return
	}

	body := m.asideContent(service, width)

	m.ui.asideCache.key = key
	m.ui.asideCache.content = body

	m.ui.asideViewport.SetContent(body)
	m.ui.asideLines = padAsideLines(strings.Split(body, "\n"), width)
}

// padAsideLines right-pads each line to width (ANSI-aware) so the render path skips per-line measurement
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
func (m Model) asideContentCacheKey(service *model.Service, innerWidth int) string {
	if service == nil {
		return strconv.Itoa(innerWidth) + "|" + string(m.state.asideTab) + "|nil"
	}

	return strconv.Itoa(innerWidth) + "|" + string(m.state.asideTab) + "|" + service.ID + "|" + string(service.Status) + "|" + strconv.Itoa(service.Process.PID) + "|" + service.Error + "|" + service.LifecycleAt.String() + "|" + m.state.now.Truncate(time.Second).String()
}

// asideScrollIndicator returns a small percent string when the aside content exceeds the viewport, otherwise empty
func (m Model) asideScrollIndicator() string {
	if m.ui.asideViewport.AtTop() && m.ui.asideViewport.AtBottom() {
		return ""
	}

	percent := m.asideScrollPercent()

	return m.theme.PanelMutedStyle.Render(strconv.Itoa(percent) + "%")
}

// asideScrollPercent returns the scroll position as a 0-100 integer so single-line scrolls rarely invalidate the cache
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
func (m Model) asideContent(service *model.Service, innerWidth int) string {
	if service == nil {
		return terminal.ContentTopMarginStyle.Render(m.theme.PlaceholderStyle.Render("no service selected"))
	}

	return terminal.ContentTopMarginStyle.Render(m.asideTabContent(service, innerWidth))
}

// asideTabContent dispatches rendering based on the active tab
func (m Model) asideTabContent(service *model.Service, innerWidth int) string {
	switch m.state.asideTab {
	case AsideTabHealth:
		return m.asideHealthTab(service, innerWidth)
	case AsideTabEnv:
		return m.asideEnvTab(service, innerWidth)
	default:
		return m.asideConfigTab(service, innerWidth)
	}
}

// asideConfigTab renders the config tab content (meta, readiness, logs, watch cards)
func (m Model) asideConfigTab(service *model.Service, innerWidth int) string {
	parts := make([]string, 0, 6)

	if errorCard := m.asideErrorCard(service, innerWidth); errorCard != "" {
		parts = append(parts, errorCard)
	}

	if metaCard := m.asideMetaCard(service, innerWidth); metaCard != "" {
		parts = append(parts, metaCard)
	}

	if readinessCard := m.asideReadinessCard(service, innerWidth); readinessCard != "" {
		parts = append(parts, readinessCard)
	}

	parts = append(parts, m.asideLogsCard(service, innerWidth))

	if watchCard := m.asideWatchCard(service, innerWidth); watchCard != "" {
		parts = append(parts, watchCard)
	}

	return strings.Join(parts, "\n\n")
}

// readinessRows returns the rows every readiness card shares: the type and the endpoint, URL over Address over Pattern
func (m Model) readinessRows(r *model.Readiness) []cardRow {
	rows := []cardRow{
		{label: "type", value: string(r.Type), style: m.theme.StatusRunningStyle},
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

// asideStatusStyle returns the style used for the status badge
func (m Model) asideStatusStyle(status model.Status) lipgloss.Style {
	switch status {
	case model.StatusPending:
		return m.theme.StatusPendingStyle
	case model.StatusRunning:
		return m.theme.StatusRunningStyle
	case model.StatusStarting, model.StatusRestarting, model.StatusStopping:
		return m.theme.StatusStartingStyle
	case model.StatusFailed:
		return m.theme.StatusFailedStyle
	case model.StatusStopped:
		return m.theme.StatusStoppedStyle
	default:
		return m.theme.PanelMutedStyle
	}
}

// asideMetaCard renders the tier, dir, and command rows in a card
func (m Model) asideMetaCard(service *model.Service, innerWidth int) string {
	rows := make([]cardRow, 0, 3)

	if service.Tier != "" {
		rows = append(rows, cardRow{label: "tier", value: service.Tier})
	}

	if service.Directory != "" {
		rows = append(rows, cardRow{label: "dir", value: service.Directory})
	}

	rows = append(rows, cardRow{label: "command", value: service.Command})

	return m.asideSection("meta", rows, innerWidth)
}

// asideErrorCard renders the friendly error reason when service.Error is set
func (m Model) asideErrorCard(service *model.Service, innerWidth int) string {
	if service.Error == "" {
		return ""
	}

	rows := []cardRow{
		{label: "reason", value: renderError(service.Error), style: m.theme.StatusFailedStyle},
	}

	return m.asideSection("error", rows, innerWidth)
}

// asideReadinessCard renders the readiness check details when configured
func (m Model) asideReadinessCard(service *model.Service, innerWidth int) string {
	if service.Readiness == nil || service.Readiness.Type == "" {
		return ""
	}

	return m.asideSection("readiness", m.readinessRows(service.Readiness), innerWidth)
}

// asideLogsCard renders configured per-service log outputs in a card
func (m Model) asideLogsCard(service *model.Service, innerWidth int) string {
	rows := []cardRow{
		{label: "output", value: strings.Join(service.LogOutput, ", ")},
	}

	return m.asideSection("logs", rows, innerWidth)
}

// asideWatchCard renders watch configuration in a card
func (m Model) asideWatchCard(service *model.Service, innerWidth int) string {
	if service.Watch == nil {
		return ""
	}

	w := service.Watch
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

// asideSection renders an optional title over indented label-value rows
func (m Model) asideSection(title string, rows []cardRow, innerWidth int) string {
	rowAvailable := asideContentWidth(innerWidth)

	labelWidth := computeLabelWidth(rows)
	lines := make([]string, 0, len(rows)+1)

	if title != "" {
		lines = append(lines, asideTitleIndent+m.theme.AsideSectionTitleStyle.Render(title))
	}

	for _, row := range rows {
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
		value = ansi.Truncate(value, remaining, "…")
	}

	return labelBlock + row.style.Render(value)
}

// asideContentWidth returns innerWidth minus the leading indent and matching right gutter
func asideContentWidth(innerWidth int) int {
	return innerWidth - 2*len(asideRowIndent)
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
