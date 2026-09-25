package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
)

// renderTier renders a tier header and its service rows
func (m Model) renderTier(tier *model.Tier, currentIdx *int) string {
	rowWidth := m.getRowWidth()
	rows := make([]string, 0, len(tier.Services)+1)

	rows = append(rows, terminal.TierHeaderStyle.Width(rowWidth).Render(tier.Name))

	for _, service := range tier.Services {
		isSelected := *currentIdx == m.state.selected
		rows = append(rows, m.renderServiceRow(service, isSelected))

		*currentIdx++
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	return terminal.TierContainerStyle.Render(content)
}

// getServiceIndicator returns the selection or status indicator for a service
func (m Model) getServiceIndicator(service *model.Service, isSelected bool) string {
	defaultIndicator := terminal.IndicatorEmpty
	if isSelected {
		defaultIndicator = terminal.IndicatorSelected
	}

	if service.Status == model.StatusRunning && service.Watching {
		return m.getWatchIndicator(isSelected)
	}

	if !transitional[service.Status] {
		return defaultIndicator
	}

	view := m.state.views[service.ID]
	if view == nil {
		return defaultIndicator
	}

	if isSelected {
		return view.Blink.Frame()
	}

	return view.Blink.Render(m.theme.IndicatorActiveStyle)
}

// getWatchIndicator returns the styled watch indicator
func (m Model) getWatchIndicator(isSelected bool) string {
	if isSelected {
		return terminal.IndicatorDot
	}

	return m.theme.IndicatorDotStyle.Render(terminal.IndicatorDot)
}

// renderTimeline renders the timeline strip for a service
func (m Model) renderTimeline(service *model.Service, isSelected bool) string {
	tw := m.ui.layout.TimelineWidth
	if tw == 0 {
		return ""
	}

	view := m.state.views[service.ID]
	if view == nil {
		return strings.Repeat(" ", tw)
	}

	slots := view.Timeline.slots()
	count := view.Timeline.Count()

	visibleObserved := min(tw, count)
	emptyPad := tw - visibleObserved
	observedStart := count - visibleObserved

	blocks := m.timelineSlotBlocks(isSelected)

	var b strings.Builder

	for i := observedStart; i < observedStart+visibleObserved; i++ {
		b.WriteString(blocks[slots[i]])
	}

	emptyBlock := blocks[TimelineSlotEmpty]
	for range emptyPad {
		b.WriteString(emptyBlock)
	}

	return b.String()
}

// timelineSlotBlocks pre-renders the styled timeline block per selection state, sparing the inner loop a style.Render
func (m Model) timelineSlotBlocks(isSelected bool) [5]string {
	return [5]string{
		TimelineSlotEmpty:    m.timelineSlotStyle(TimelineSlotEmpty, isSelected).Render(terminal.TimelineBlock),
		TimelineSlotRunning:  m.timelineSlotStyle(TimelineSlotRunning, isSelected).Render(terminal.TimelineBlock),
		TimelineSlotStarting: m.timelineSlotStyle(TimelineSlotStarting, isSelected).Render(terminal.TimelineBlock),
		TimelineSlotFailed:   m.timelineSlotStyle(TimelineSlotFailed, isSelected).Render(terminal.TimelineBlock),
		TimelineSlotStopped:  m.timelineSlotStyle(TimelineSlotStopped, isSelected).Render(terminal.TimelineBlock),
	}
}

// timelineSlotStyle returns the style for a timeline slot, one of the TimelineSelected styles when the row is selected
func (m Model) timelineSlotStyle(slot TimelineSlot, isSelected bool) lipgloss.Style {
	if isSelected {
		switch slot {
		case TimelineSlotRunning:
			return m.theme.TimelineSelectedRunningStyle
		case TimelineSlotStarting:
			return m.theme.TimelineSelectedStartingStyle
		case TimelineSlotFailed:
			return m.theme.TimelineSelectedFailedStyle
		case TimelineSlotStopped:
			return m.theme.TimelineSelectedStoppedStyle
		default:
			return m.theme.TimelineSelectedEmptyStyle
		}
	}

	switch slot {
	case TimelineSlotRunning:
		return m.theme.TimelineRunningStyle
	case TimelineSlotStarting:
		return m.theme.TimelineStartingStyle
	case TimelineSlotFailed:
		return m.theme.TimelineFailedStyle
	case TimelineSlotStopped:
		return m.theme.TimelineStoppedStyle
	default:
		return m.theme.TimelineEmptyStyle
	}
}

// renderServiceRow renders a single service row with all columns
func (m Model) renderServiceRow(service *model.Service, isSelected bool) string {
	rowWidth := m.getRowWidth()
	indicator := m.getServiceIndicator(service, isSelected)

	nameTextWidth := max(m.ui.layout.ServiceNameWidth-terminal.IndicatorColumnWidth-terminal.ServiceNameTrailingGap, 1)
	name := terminal.TruncateAndPad(service.Name, nameTextWidth)
	nameCol := fmt.Sprintf("%s %s ", indicator, name)

	style := terminal.ServiceRowStyle
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
		hasError:   service.Error != "",
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
		tail = terminal.PadRight(timelineGap+parts.status+parts.details, remaining)
	}

	if parts.isSelected && m.ui.layout.TimelineWidth > 0 {
		tail = m.theme.SelectionBgStyle.Render(tail)
	}

	return parts.name + leftFlex + parts.timeline + tail
}

// getServiceDetails returns either error message or metrics columns
func (m Model) getServiceDetails(service *model.Service, isSelected bool) string {
	if service.Error != "" && isSelected {
		return terminal.RowErrorPadding + renderError(service.Error)
	}

	if service.Error != "" {
		return m.theme.ErrorStyle.Render(terminal.RowErrorPadding + renderError(service.Error))
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
		fmt.Fprintf(&b, "%*s", w, ansi.Truncate(values[i], w, "…"))
	}

	return b.String()
}

// styledStatus returns the status string with the lifecycle-appropriate color (no padding)
func (m Model) styledStatus(service *model.Service, isSelected bool) string {
	statusStr := string(service.Status)
	if isSelected {
		return statusStr
	}

	return m.statusStyle(service.Status).Render(statusStr)
}

// getStyledAndPaddedStatus returns the styled status string padded to fit StatusWidth
func (m Model) getStyledAndPaddedStatus(service *model.Service, isSelected bool) string {
	statusStr := string(service.Status)
	paddingLen := max(m.ui.layout.StatusWidth-len(statusStr), 0)
	padding := strings.Repeat(terminal.IndicatorEmpty, paddingLen)

	return m.styledStatus(service, isSelected) + padding
}
