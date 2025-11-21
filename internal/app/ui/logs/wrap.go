package logs

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/ansi"
)

const (
	// maxWastedSpace is the maximum amount of line space we're willing to waste
	maxWastedSpace = 20
)

// charInfo tracks display width and byte position for each character
type charInfo struct {
	byteOffset int // Starting byte offset in original string
	width      int // Display width (0 for ANSI, 1 for ASCII, 2 for wide chars)
	isSpace    bool
}

// wrapText wraps text to fit within maxWidth by display width (ANSI-aware)
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

// buildCharTable creates a table mapping character positions to byte offsets and display widths
func buildCharTable(text string) []charInfo {
	if len(text) == 0 {
		return nil
	}

	var chars []charInfo

	bytePos := 0

	for bytePos < len(text) {
		if text[bytePos] == '\x1b' {
			seqLen := 1
			for bytePos+seqLen < len(text) {
				ch := text[bytePos+seqLen]
				seqLen++
				if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
					break
				}
			}

			chars = append(chars, charInfo{
				byteOffset: bytePos,
				width:      0,
				isSpace:    false,
			})

			bytePos += seqLen

			continue
		}

		r, size := utf8.DecodeRuneInString(text[bytePos:])
		if r == utf8.RuneError {
			chars = append(chars, charInfo{
				byteOffset: bytePos,
				width:      1,
				isSpace:    false,
			})
			bytePos++

			continue
		}

		width := ansi.PrintableRuneWidth(string(r))
		isSpace := r == ' ' || r == '\t'

		chars = append(chars, charInfo{
			byteOffset: bytePos,
			width:      width,
			isSpace:    isSpace,
		})

		bytePos += size
	}

	return chars
}

// findBreakPoint finds the byte offset where text should be broken to fit within maxWidth
func findBreakPoint(text string, maxWidth int) int {
	if len(text) == 0 {
		return 0
	}

	chars := buildCharTable(text)
	if len(chars) == 0 {
		return 0
	}

	currentWidth := 0
	lastSpaceIdx := -1
	lastSpaceWidth := 0

	for i, ch := range chars {
		newWidth := currentWidth + ch.width

		if newWidth > maxWidth {
			if lastSpaceIdx >= 0 {
				wastedSpace := maxWidth - lastSpaceWidth
				if wastedSpace <= maxWastedSpace {
					if lastSpaceIdx+1 < len(chars) {
						return chars[lastSpaceIdx+1].byteOffset
					}

					return len(text)
				}
			}

			if i > 0 {
				return chars[i].byteOffset
			}

			if i+1 < len(chars) {
				return chars[i+1].byteOffset
			}

			return len(text)
		}

		currentWidth = newWidth

		if ch.isSpace {
			lastSpaceIdx = i
			lastSpaceWidth = currentWidth
		}
	}

	return len(text)
}
