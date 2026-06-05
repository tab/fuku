package components

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

// RenderPanel renders a bordered panel with titles in the borders
func RenderPanel(opts PanelOptions) string {
	return strings.Join(RenderPanelLines(opts), "\n")
}

// RenderPanelLines returns the panel as a slice of lines so callers can join them with sibling panels by direct concatenation, avoiding the lipgloss.JoinHorizontal ANSI-width pass
func RenderPanelLines(opts PanelOptions) []string {
	innerWidth := opts.Width - PanelInnerPadding

	titleText := PanelTitleStyle.Render(opts.Title)

	borderStyle := opts.BorderStyle
	if borderStyle.GetForeground() == nil {
		borderStyle = PanelBorderStyle
	}

	border := func(s string) string { return borderStyle.Render(s) }

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

// RenderFooter renders the help/tips footer row at the given width
func RenderFooter(help, tips string, width int) string {
	return renderFooter(help, tips, width)
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

// ComputeCompactLayout returns a layout for the aside-open mode that allocates space to the service name (sized for the longest name) and a right-aligned status column, with no timeline or metrics
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

// ComputeTableLayout returns column widths based on the available content width, preferred name text width, and number of metric columns to reserve
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

// fitBorderText truncates left and right text so the border line fits within
// innerWidth, accounting for spacers, edge borders and the minimum middle
// filler; either side may be empty and the other side is given the full budget
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
	spacer := PanelTitleSpacer.Render("")
	leftSpacer, rightSpacer := SplitAtDisplayWidth(spacer)

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
		result += leftSpacer + leftText + rightSpacer
		result += border(hLine(fillWidth))
	} else {
		result = border(leftCorner + hLine(fillWidth+leftLen))
	}

	if rightText != "" {
		result += leftSpacer + rightText + rightSpacer
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

// AppendContentLines adds content lines with borders and padding; the bordered vertical glyph is rendered once and reused across every row
func AppendContentLines(result, contentLines []string, innerWidth int, border func(string) string) []string {
	verticalBorder := border(BorderVertical)

	for _, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		padding := max(innerWidth-lineWidth, 0)

		paddedLine := line + strings.Repeat(IndicatorEmpty, padding)
		result = append(result, verticalBorder+paddedLine+verticalBorder)
	}

	return result
}

// AppendPrePaddedContentLines is a faster variant of AppendContentLines that assumes the input lines are already padded to innerWidth, skipping the per-line lipgloss.Width call
func AppendPrePaddedContentLines(result, contentLines []string, border func(string) string) []string {
	verticalBorder := border(BorderVertical)

	for _, line := range contentLines {
		result = append(result, verticalBorder+line+verticalBorder)
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
