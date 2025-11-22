package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"fuku/internal/config"
)

// RenderHeader renders app header
func RenderHeader(width int, title, info string) string {
	titleWidth := lipgloss.Width(title)
	infoWidth := lipgloss.Width(info)

	maxTitleWidth := width - infoWidth - HeaderSeparatorMinWidth - HeaderFixedChars
	if titleWidth > maxTitleWidth && maxTitleWidth > 0 {
		title = truncate(title, maxTitleWidth)
		titleWidth = lipgloss.Width(title)
	}

	separatorWidth := width - titleWidth - infoWidth - HeaderFixedChars
	if separatorWidth < HeaderSeparatorMinWidth {
		separatorWidth = HeaderSeparatorMinWidth
	}

	leftPrefix := RenderLine(3)
	separator := RenderLine(separatorWidth)
	rightSuffix := RenderLine(3)

	return HeaderStyle.Render(leftPrefix + " " + title + " " + separator + " " + info + " " + rightSuffix)
}

// RenderFooter renders app footer
func RenderFooter(width int, helpText string) string {
	version := fmt.Sprintf("v%s", config.Version)
	versionWidth := lipgloss.Width(version)

	separatorWidth := width - versionWidth - FooterFixedChars
	if separatorWidth < FooterSeparatorMinWidth {
		separatorWidth = FooterSeparatorMinWidth
	}

	leftSeparator := RenderLine(separatorWidth)
	rightSuffix := RenderLine(3)
	versionLine := leftSeparator + " " + version + " " + rightSuffix

	help := FooterHelpStyle.Render(HelpStyle.Render(helpText))

	return FooterStyle.Render(lipgloss.JoinVertical(lipgloss.Left, versionLine, help))
}

// RenderLine renders a horizontal line of the specified width with separator style
func RenderLine(width int) string {
	if width < 0 {
		width = 0
	}

	return SeparatorStyle.Render(strings.Repeat("─", width))
}

func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	if maxWidth == 1 {
		return "…"
	}

	if lipgloss.Width(s) <= maxWidth {
		return s
	}

	runes := []rune(s)
	for i := len(runes) - 1; i >= 0; i-- {
		truncated := string(runes[:i]) + "…"
		if lipgloss.Width(truncated) <= maxWidth {
			return truncated
		}
	}

	return "…"
}
