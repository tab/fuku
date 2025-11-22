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
		title = Truncate(title, maxTitleWidth)
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
