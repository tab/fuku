package terminal

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// PanelOptions contains options for rendering a panel
type PanelOptions struct {
	Title       string
	Content     string
	Status      string
	Stats       string
	Version     string
	Height      int
	Width       int
	BorderStyle lipgloss.Style
}

// RenderPanelLines returns the panel as lines, so callers concatenate sibling panels without lipgloss.JoinHorizontal
func RenderPanelLines(opts PanelOptions) []string {
	innerWidth := opts.Width - PanelInnerPadding

	titleText := PanelTitleStyle.Render(opts.Title)

	border := func(s string) string { return opts.BorderStyle.Render(s) }

	topBorder := BuildTopBorder(border, titleText, opts.Status, innerWidth)
	bottomBorder := BuildBottomBorder(border, opts.Stats, opts.Version, innerWidth)

	contentHeight := max(opts.Height-PanelBorderHeight, 1)

	contentLines := SplitAndPadContent(opts.Content, contentHeight)

	lines := make([]string, 0, contentHeight+2)
	lines = append(lines, topBorder)
	lines = AppendContentLines(lines, contentLines, innerWidth, border)
	lines = append(lines, bottomBorder)

	return lines
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
	MetricColumns    int
}

// PreferredNameTextWidth picks a bucket value based on the longest service name length
func PreferredNameTextWidth(name int) int {
	switch {
	case name <= ServiceNameWidthShort:
		return ServiceNameWidthShort
	case name <= ServiceNameWidthMedium:
		return ServiceNameWidthMedium
	case name <= ServiceNameWidthLong:
		return ServiceNameWidthLong
	default:
		return name + ServiceNameTrailingGap
	}
}

// ComputeCompactLayout returns the aside-open layout: the longest name plus a status column, no timeline or metrics
func ComputeCompactLayout(contentWidth, preferredNameTextWidth int) TableLayout {
	if contentWidth < 0 {
		contentWidth = 0
	}

	statusWidth := min(StatusCompactWidth, contentWidth)

	nameColWidth := preferredNameTextWidth + IndicatorColumnWidth + ServiceNameTrailingGap
	if nameColWidth+statusWidth > contentWidth {
		nameColWidth = max(contentWidth-statusWidth, 0)
	}

	leftFlex := max(contentWidth-nameColWidth-statusWidth, 0)

	return TableLayout{
		ContentWidth:     contentWidth,
		ServiceNameWidth: nameColWidth,
		LeftFlexWidth:    leftFlex,
		StatusWidth:      statusWidth,
	}
}

// ComputeTableLayout returns column widths for the content width, the preferred name width and the metric column count
func ComputeTableLayout(contentWidth, preferredNameTextWidth, metricColumns int) TableLayout {
	if contentWidth < 0 {
		contentWidth = 0
	}

	if metricColumns < 0 {
		metricColumns = 0
	}

	preferredNameWidth := IndicatorColumnWidth + preferredNameTextWidth

	statusWidth := min(contentWidth/StatusWidthDivisor, StatusMaxWidth)

	metricWidth := 0
	if metricColumns > 0 {
		metricWidth = min(contentWidth/MetricWidthDivisor, MetricMaxWidth)
	}

	available := contentWidth - statusWidth - metricColumns*metricWidth

	serviceNameWidth, timelineWidth, gap := allocateNameAndTimeline(available, preferredNameWidth)

	used := serviceNameWidth + timelineWidth + gap + statusWidth + metricColumns*metricWidth
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
		MetricColumns:    metricColumns,
	}
}

// allocateNameAndTimeline distributes available width between name and timeline columns
func allocateNameAndTimeline(available, preferredNameWidth int) (name, timeline, gap int) {
	if available <= 0 {
		return 0, 0, 0
	}

	fullTotal := preferredNameWidth + TimelineDefaultSlots + TimelineGap
	if available >= fullTotal {
		return preferredNameWidth, TimelineDefaultSlots, TimelineGap
	}

	preferredNameWithMinTimeline := preferredNameWidth + TimelineMinWidth + TimelineGap
	if available >= preferredNameWithMinTimeline {
		return preferredNameWidth, available - preferredNameWidth - TimelineGap, TimelineGap
	}

	nameWithMinTimeline := available - TimelineMinWidth - TimelineGap
	if nameWithMinTimeline >= ServiceNameMinWidth {
		return nameWithMinTimeline, TimelineMinWidth, TimelineGap
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

// TruncateAndPad truncates text to width with an ellipsis or pads it to exactly that display width
func TruncateAndPad(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return PadRight(s, width)
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

// fitBorderText shortens the left and right text so the border line, its spacers and edges fit innerWidth
func fitBorderText(leftText, rightText string, innerWidth int) (left, right string) {
	hasLeft := leftText != ""
	hasRight := rightText != ""

	if !hasLeft && !hasRight {
		return leftText, rightText
	}

	budget := innerWidth - BorderSpacerWidth - BorderEdgeWidth - 1
	if hasLeft && hasRight {
		budget -= BorderSpacerWidth + BorderEdgeWidth
	}

	if budget <= 0 {
		return "", ""
	}

	leftWidth := lipgloss.Width(leftText)
	rightWidth := lipgloss.Width(rightText)

	if leftWidth+rightWidth <= budget {
		return leftText, rightText
	}

	if !hasLeft {
		return leftText, ansi.Truncate(rightText, budget, "…")
	}

	if !hasRight {
		return ansi.Truncate(leftText, budget, "…"), rightText
	}

	half := budget / 2

	if leftWidth <= half {
		return leftText, ansi.Truncate(rightText, budget-leftWidth, "…")
	}

	if rightWidth <= half {
		return ansi.Truncate(leftText, budget-rightWidth, "…"), rightText
	}

	return ansi.Truncate(leftText, half, "…"), ansi.Truncate(rightText, budget-half, "…")
}

// BuildTopBorder builds the top border with title and optional right-side text
func BuildTopBorder(border func(string) string, titleText, topRightText string, innerWidth int) string {
	return buildBorderLine(border, BorderTopLeft, BorderTopRight, titleText, topRightText, innerWidth)
}

// BuildBottomBorder builds the bottom border with optional info (left) and version (right)
func BuildBottomBorder(border func(string) string, bottomLeftText, bottomRightText string, innerWidth int) string {
	return buildBorderLine(border, BorderBottomLeft, BorderBottomRight, bottomLeftText, bottomRightText, innerWidth)
}

// buildBorderLine renders a horizontal border with optional left-side and right-side text segments
func buildBorderLine(border func(string) string, leftCorner, rightCorner, leftText, rightText string, innerWidth int) string {
	hLine := func(n int) string { return strings.Repeat(BorderHorizontal, n) }

	leftText, rightText = fitBorderText(leftText, rightText, innerWidth)

	leftLen := 0
	if leftText != "" {
		leftLen = lipgloss.Width(leftText) + BorderSpacerWidth + BorderEdgeWidth
	}

	rightLen := 0
	if rightText != "" {
		rightLen = lipgloss.Width(rightText) + BorderSpacerWidth + BorderEdgeWidth
	}

	fillWidth := max(innerWidth-leftLen-rightLen, 1)

	var result string

	if leftText != "" {
		result = border(leftCorner + hLine(BorderEdgeWidth))
		result += IndicatorEmpty + leftText + IndicatorEmpty
		result += border(hLine(fillWidth))
	} else {
		result = border(leftCorner + hLine(fillWidth+leftLen))
	}

	if rightText != "" {
		result += IndicatorEmpty + rightText + IndicatorEmpty
		result += border(hLine(BorderEdgeWidth))
	}

	result += border(rightCorner)

	return result
}

// SplitAndPadContent splits content into lines and pads or truncates to fill height
func SplitAndPadContent(content string, height int) []string {
	lines := strings.Split(content, "\n")

	for len(lines) < height {
		lines = append(lines, "")
	}

	if len(lines) > height {
		lines = lines[:height]
	}

	return lines
}

// AppendContentLines adds content lines with borders and padding (the vertical border glyph is rendered once)
func AppendContentLines(result, contentLines []string, innerWidth int, border func(string) string) []string {
	verticalBorder := border(BorderVertical)

	for _, line := range contentLines {
		result = append(result, verticalBorder+PadRight(line, innerWidth)+verticalBorder)
	}

	return result
}

// AppendPrePaddedContentLines is AppendContentLines for lines already padded to innerWidth, skipping their measurement
func AppendPrePaddedContentLines(result, contentLines []string, border func(string) string) []string {
	verticalBorder := border(BorderVertical)

	for _, line := range contentLines {
		result = append(result, verticalBorder+line+verticalBorder)
	}

	return result
}

// RenderFooter renders the footer with help and tips
func RenderFooter(help, tips string, width int) string {
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
