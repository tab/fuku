package logs

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// wrapText wraps text to fit within maxWidth by display width (ANSI-aware)
// Prefers to break at whitespace and returns lines without trailing spaces
func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}

	displayWidth := lipgloss.Width(text)
	if displayWidth <= maxWidth {
		return []string{text}
	}

	var lines []string

	for len(text) > 0 {
		visibleWidth := lipgloss.Width(text)
		if visibleWidth <= maxWidth {
			lines = append(lines, text)
			break
		}

		breakPoint := findBreakPoint(text, maxWidth)

		line := text[:breakPoint]
		line = strings.TrimRight(line, " \t")
		lines = append(lines, line)

		text = text[breakPoint:]
		text = strings.TrimLeft(text, " \t")
	}

	return lines
}

func findBreakPoint(text string, maxWidth int) int {
	if len(text) == 0 {
		return 0
	}

	lastSpace := -1
	lastSpaceWidth := 0
	bestPos := 0

	for i := 1; i <= len(text); i++ {
		chunk := text[:i]
		width := lipgloss.Width(chunk)

		if width > maxWidth {
			if lastSpace > 0 {
				wastedSpace := maxWidth - lastSpaceWidth
				if wastedSpace <= 20 {
					return lastSpace
				}
			}

			if bestPos == 0 {
				return 1
			}

			return bestPos
		}

		if i <= len(text) && (text[i-1] == ' ' || text[i-1] == '\t') {
			lastSpace = i
			lastSpaceWidth = width
		}

		bestPos = i
	}

	return len(text)
}
