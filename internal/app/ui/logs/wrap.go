package logs

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/ansi"
)

const (
	// maxWastedSpace is the maximum amount of line space we're willing to waste
	// when breaking at whitespace. If breaking at a space would leave more than
	// this many columns unused, we'll break mid-word instead to avoid excessive
	// whitespace at line ends.
	maxWastedSpace = 20
)

// charInfo tracks display width and byte position for each character
type charInfo struct {
	byteOffset int // Starting byte offset in original string
	width      int // Display width (0 for ANSI, 1 for ASCII, 2 for wide chars)
	isSpace    bool
}

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

// buildCharTable creates a table mapping character positions to byte offsets and display widths.
// This enables O(n) wrapping by avoiding repeated width calculations.
func buildCharTable(text string) []charInfo {
	if len(text) == 0 {
		return nil
	}

	var chars []charInfo

	bytePos := 0

	for bytePos < len(text) {
		// Check for ANSI escape sequence
		if text[bytePos] == '\x1b' {
			// Find the end of the ANSI sequence
			seqLen := 1
			for bytePos+seqLen < len(text) {
				ch := text[bytePos+seqLen]
				seqLen++
				// ANSI sequences end with a letter (simplified CSI parsing)
				if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
					break
				}
			}

			// ANSI sequences have zero display width
			chars = append(chars, charInfo{
				byteOffset: bytePos,
				width:      0,
				isSpace:    false,
			})

			bytePos += seqLen

			continue
		}

		// Decode UTF-8 rune
		r, size := utf8.DecodeRuneInString(text[bytePos:])
		if r == utf8.RuneError {
			// Invalid UTF-8, treat as single byte
			chars = append(chars, charInfo{
				byteOffset: bytePos,
				width:      1,
				isSpace:    false,
			})
			bytePos++

			continue
		}

		// Determine display width
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

// findBreakPoint finds the byte offset where text should be broken to fit within maxWidth.
// Uses precomputed character widths for O(n) performance.
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

		// Check if adding this character would exceed width
		if newWidth > maxWidth {
			// We've exceeded maxWidth, decide where to break
			if lastSpaceIdx >= 0 {
				wastedSpace := maxWidth - lastSpaceWidth
				if wastedSpace <= maxWastedSpace {
					// Break after the space
					if lastSpaceIdx+1 < len(chars) {
						return chars[lastSpaceIdx+1].byteOffset
					}

					return len(text)
				}
			}

			// Break at current position (don't include this char)
			if i > 0 {
				return chars[i].byteOffset
			}

			// First character doesn't fit, must include it anyway
			if i+1 < len(chars) {
				return chars[i+1].byteOffset
			}

			return len(text)
		}

		// This character fits
		currentWidth = newWidth

		if ch.isSpace {
			lastSpaceIdx = i
			lastSpaceWidth = currentWidth
		}
	}

	return len(text)
}
