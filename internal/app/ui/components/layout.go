package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	titleLen := lipgloss.Width(titleText)
	statusLen := lipgloss.Width(opts.Status) + SpacerWidth + BorderEdgeWidth

	middleLineWidth := innerWidth - titleLen - statusLen - BorderEdgeWidth - SpacerWidth
	if middleLineWidth < 1 {
		middleLineWidth = 1
	}

	border := func(s string) string { return PanelBorderStyle.Render(s) }

	topBorder := buildTopBorder(border, titleText, opts.Status, middleLineWidth)
	bottomBorder := buildBottomBorder(border, opts.Stats, opts.Version, innerWidth)

	contentHeight := opts.Height - PanelBorderHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	contentLines := splitAndPadContent(opts.Content, contentHeight)

	lines := make([]string, 0, contentHeight+3)
	lines = append(lines, topBorder)
	lines = appendContentLines(lines, contentLines, innerWidth, border)
	lines = append(lines, bottomBorder)

	panel := lipgloss.JoinVertical(lipgloss.Left, lines...)
	footer := renderFooter(opts.Help, opts.Tips, opts.Width)

	return lipgloss.JoinVertical(lipgloss.Left, panel, footer)
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

// Truncate truncates text to fit within maxWidth display columns
func Truncate(s string, maxWidth int) string {
	currentWidth := lipgloss.Width(s)
	if currentWidth <= maxWidth {
		return s
	}

	ellipsis := "…"
	ellipsisWidth := lipgloss.Width(ellipsis)
	targetWidth := maxWidth - ellipsisWidth

	if targetWidth <= 0 {
		return ellipsis
	}

	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i])
		if lipgloss.Width(candidate) <= targetWidth {
			return candidate + ellipsis
		}
	}

	return ellipsis
}

// buildTopBorder builds the top border with title and status
func buildTopBorder(border func(string) string, titleText, topRightText string, middleWidth int) string {
	hLine := func(n int) string { return strings.Repeat(BorderHorizontal, n) }
	spacer := PanelTitleSpacer.Render("")
	leftSpacer, rightSpacer := splitAtDisplayWidth(spacer)

	result := border(BorderTopLeft + hLine(BorderEdgeWidth))
	result += leftSpacer + titleText + rightSpacer
	result += border(hLine(middleWidth))
	result += leftSpacer + topRightText + rightSpacer
	result += border(hLine(BorderEdgeWidth))
	result += border(BorderTopRight)

	return result
}

// buildBottomBorder builds the bottom border with optional info (left) and version (right)
func buildBottomBorder(border func(string) string, bottomLeftText, bottomRightText string, innerWidth int) string {
	hLine := func(n int) string { return strings.Repeat(BorderHorizontal, n) }

	spacer := PanelTitleSpacer.Render("")
	leftSpacer, rightSpacer := splitAtDisplayWidth(spacer)

	rightText := PanelMutedStyle.Render(bottomRightText)
	rightLen := lipgloss.Width(rightText) + SpacerWidth + BorderEdgeWidth

	leftLen := 0
	leftText := ""

	if bottomLeftText != "" {
		leftText = PanelMutedStyle.Render(bottomLeftText)
		leftLen = lipgloss.Width(leftText) + SpacerWidth + BorderEdgeWidth
	}

	middleWidth := innerWidth - leftLen - rightLen
	if middleWidth < 1 {
		middleWidth = 1
	}

	var result string

	if leftText != "" {
		result = border(BorderBottomLeft + hLine(BorderEdgeWidth))
		result += leftSpacer + leftText + rightSpacer
		result += border(hLine(middleWidth))
	} else {
		result = border(BorderBottomLeft + hLine(middleWidth+leftLen))
	}

	result += leftSpacer + rightText + rightSpacer
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

// appendContentLines adds content lines with borders and padding
func appendContentLines(result, contentLines []string, innerWidth int, border func(string) string) []string {
	for _, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		padding := innerWidth - lineWidth

		if padding < 0 {
			padding = 0
		}

		paddedLine := line + strings.Repeat(IndicatorEmpty, padding)
		result = append(result, border(BorderVertical)+paddedLine+border(BorderVertical))
	}

	return result
}

// splitAtDisplayWidth splits a string at half its display width
func splitAtDisplayWidth(s string) (left, right string) {
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
