package components

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// PanelOptions contains options for rendering a panel
type PanelOptions struct {
	Title   string
	Content string
	Status  string
	Stats   string
	Version string
	Help    string
	Tips    string
	Height  int
	Width   int
}

// RenderPanel renders a bordered panel with titles in the borders
func RenderPanel(opts PanelOptions) string {
	innerWidth := opts.Width - PanelInnerPadding

	titleText := PanelTitleStyle.Render(opts.Title)

	border := func(s string) string { return PanelBorderStyle.Render(s) }

	topBorder := BuildTopBorder(border, titleText, opts.Status, innerWidth)
	bottomBorder := BuildBottomBorder(border, opts.Stats, opts.Version, innerWidth)

	contentHeight := max(opts.Height-PanelBorderHeight, 1)

	contentLines := splitAndPadContent(opts.Content, contentHeight)

	lines := make([]string, 0, contentHeight+3)
	lines = append(lines, topBorder)
	lines = AppendContentLines(lines, contentLines, innerWidth, border)
	lines = append(lines, bottomBorder)

	panel := lipgloss.JoinVertical(lipgloss.Left, lines...)
	footer := renderFooter(opts.Help, opts.Tips, opts.Width)

	return lipgloss.JoinVertical(lipgloss.Left, panel, footer)
}

// TableLayout holds computed column widths for the services table
type TableLayout struct {
	ContentWidth     int
	ServiceNameWidth int
	LeftFlexWidth    int
	TimelineWidth    int
	TimelineGapWidth int
	StatusWidth      int
	RightFlexWidth   int
	MetricWidth      int
}

// PreferredNameTextWidth picks a bucket value based on the longest service name length
func PreferredNameTextWidth(name int) int {
	switch {
	case name <= NameWidthShort:
		return NameWidthShort
	case name <= NameWidthMedium:
		return NameWidthMedium
	case name <= NameWidthLong:
		return NameWidthLong
	default:
		return name + NameTrailingGap
	}
}

// ComputeTableLayout returns column widths based on the available content width and preferred name text width
func ComputeTableLayout(contentWidth, preferredNameTextWidth int) TableLayout {
	if contentWidth < 0 {
		contentWidth = 0
	}

	preferredNameWidth := IndicatorColumnWidth + preferredNameTextWidth

	statusWidth := min(contentWidth/StatusWidthDivisor, MaxStatusWidth)
	metricWidth := min(contentWidth/MetricWidthDivisor, MaxMetricWidth)

	available := contentWidth - statusWidth - MetricColumnCount*metricWidth

	serviceNameWidth, timelineWidth, gap := allocateNameAndTimeline(available, preferredNameWidth)

	used := serviceNameWidth + timelineWidth + gap + statusWidth + MetricColumnCount*metricWidth
	surplus := max(contentWidth-used, 0)
	leftFlex := surplus / 2
	rightFlex := surplus - leftFlex

	return TableLayout{
		ContentWidth:     contentWidth,
		ServiceNameWidth: serviceNameWidth,
		LeftFlexWidth:    leftFlex,
		TimelineWidth:    timelineWidth,
		TimelineGapWidth: gap,
		StatusWidth:      statusWidth,
		RightFlexWidth:   rightFlex,
		MetricWidth:      metricWidth,
	}
}

// allocateNameAndTimeline distributes available width between name and timeline columns
func allocateNameAndTimeline(available, preferredNameWidth int) (name, timeline, gap int) {
	if available <= 0 {
		return 0, 0, 0
	}

	fullTotal := preferredNameWidth + DefaultTimelineSlots + TimelineGap
	if available >= fullTotal {
		return preferredNameWidth, DefaultTimelineSlots, TimelineGap
	}

	preferredNameWithMinTimeline := preferredNameWidth + MinTimelineWidth + TimelineGap
	if available >= preferredNameWithMinTimeline {
		return preferredNameWidth, available - preferredNameWidth - TimelineGap, TimelineGap
	}

	nameWithMinTimeline := available - MinTimelineWidth - TimelineGap
	if nameWithMinTimeline >= MinServiceNameWidth {
		return nameWithMinTimeline, MinTimelineWidth, TimelineGap
	}

	return available, 0, 0
}

// PadRight pads text to width using display width (not rune count)
func PadRight(s string, width int) string {
	currentWidth := lipgloss.Width(s)
	if currentWidth >= width {
		return s
	}

	return s + strings.Repeat(IndicatorEmpty, width-currentWidth)
}

// TruncateAndPad truncates text exceeding width (with ellipsis) or pads shorter text to exactly match the specified display width
func TruncateAndPad(s string, width int) string {
	currentWidth := lipgloss.Width(s)

	if currentWidth == width {
		return s
	}

	if currentWidth < width {
		return s + strings.Repeat(IndicatorEmpty, width-currentWidth)
	}

	ellipsis := "…"
	ellipsisWidth := 1
	targetWidth := width - ellipsisWidth

	if targetWidth <= 0 {
		return ellipsis
	}

	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i])
		candidateWidth := lipgloss.Width(candidate)

		if candidateWidth <= targetWidth {
			return candidate + ellipsis + strings.Repeat(IndicatorEmpty, width-candidateWidth-ellipsisWidth)
		}
	}

	return ellipsis + strings.Repeat(IndicatorEmpty, width-ellipsisWidth)
}

// BuildTopBorder builds the top border with title and optional right-side text
func BuildTopBorder(border func(string) string, titleText, topRightText string, innerWidth int) string {
	hLine := func(n int) string { return strings.Repeat(BorderHorizontal, n) }
	spacer := PanelTitleSpacer.Render("")
	leftSpacer, rightSpacer := SplitAtDisplayWidth(spacer)

	titleLen := lipgloss.Width(titleText) + SpacerWidth + BorderEdgeWidth

	rightLen := 0
	if topRightText != "" {
		rightLen = lipgloss.Width(topRightText) + SpacerWidth + BorderEdgeWidth
	}

	fillWidth := max(innerWidth-titleLen-rightLen, 1)

	result := border(BorderTopLeft + hLine(BorderEdgeWidth))
	result += leftSpacer + titleText + rightSpacer
	result += border(hLine(fillWidth))

	if topRightText != "" {
		result += leftSpacer + topRightText + rightSpacer
		result += border(hLine(BorderEdgeWidth))
	}

	result += border(BorderTopRight)

	return result
}

// BuildBottomBorder builds the bottom border with optional info (left) and version (right)
func BuildBottomBorder(border func(string) string, bottomLeftText, bottomRightText string, innerWidth int) string {
	hLine := func(n int) string { return strings.Repeat(BorderHorizontal, n) }

	spacer := PanelTitleSpacer.Render("")
	leftSpacer, rightSpacer := SplitAtDisplayWidth(spacer)

	rightLen := lipgloss.Width(bottomRightText) + SpacerWidth + BorderEdgeWidth

	leftLen := 0

	if bottomLeftText != "" {
		leftLen = lipgloss.Width(bottomLeftText) + SpacerWidth + BorderEdgeWidth
	}

	middleWidth := max(innerWidth-leftLen-rightLen, 1)

	var result string

	if bottomLeftText != "" {
		result = border(BorderBottomLeft + hLine(BorderEdgeWidth))
		result += leftSpacer + bottomLeftText + rightSpacer
		result += border(hLine(middleWidth))
	} else {
		result = border(BorderBottomLeft + hLine(middleWidth+leftLen))
	}

	result += leftSpacer + bottomRightText + rightSpacer
	result += border(hLine(BorderEdgeWidth))
	result += border(BorderBottomRight)

	return result
}

// splitAndPadContent splits content into lines and pads to fill height
func splitAndPadContent(content string, height int) []string {
	lines := strings.Split(content, "\n")

	for len(lines) < height {
		lines = append(lines, "")
	}

	if len(lines) > height {
		lines = lines[:height]
	}

	return lines
}

// AppendContentLines adds content lines with borders and padding
func AppendContentLines(result, contentLines []string, innerWidth int, border func(string) string) []string {
	for _, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		padding := max(innerWidth-lineWidth, 0)

		paddedLine := line + strings.Repeat(IndicatorEmpty, padding)
		result = append(result, border(BorderVertical)+paddedLine+border(BorderVertical))
	}

	return result
}

// SplitAtDisplayWidth splits a string at half its display width
func SplitAtDisplayWidth(s string) (left, right string) {
	runes := []rune(s)
	totalWidth := lipgloss.Width(s)
	targetWidth := totalWidth / 2

	currentWidth := 0
	splitIdx := 0

	for i, r := range runes {
		runeWidth := lipgloss.Width(string(r))
		if currentWidth+runeWidth > targetWidth {
			splitIdx = i
			break
		}

		currentWidth += runeWidth
		splitIdx = i + 1
	}

	return string(runes[:splitIdx]), string(runes[splitIdx:])
}

// renderFooter renders the footer with help and tips
func renderFooter(help, tips string, width int) string {
	content := FooterStyle.Render(help)

	if tips == "" {
		return FooterMarginStyle.Render(content)
	}

	tipsContent := TipStyle.Render(tips)

	helpWidth := lipgloss.Width(content)
	tipsWidth := lipgloss.Width(tipsContent)
	gap := width - helpWidth - tipsWidth

	if gap < 1 {
		return FooterMarginStyle.Render(content)
	}

	row := content + strings.Repeat(IndicatorEmpty, gap) + tipsContent

	return FooterMarginStyle.Render(row)
}
