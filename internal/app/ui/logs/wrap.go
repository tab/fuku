package logs

import (
	"strings"
	"unicode/utf8"

	"github.com/muesli/ansi"
)

const (
	// maxWastedSpace is the maximum amount of line space we're willing to waste
	maxWastedSpace = 20
)

// charInfo tracks display width and byte position for each character
type charInfo struct {
	byteOffset      int // Starting byte offset in original string
	width           int // Display width (0 for ANSI, 1 for ASCII, 2 for wide chars)
	isSpace         bool
	cumulativeWidth int // Cumulative display width up to and including this character
}

// wrapText wraps text to fit within maxWidth by display width (ANSI-aware)
func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}

	chars := buildCharTable(text)
	if len(chars) == 0 {
		return []string{text}
	}

	totalWidth := chars[len(chars)-1].cumulativeWidth
	if totalWidth <= maxWidth {
		return []string{text}
	}

	var lines []string

	startIdx := 0

	for startIdx < len(chars) {
		startWidth := 0
		if startIdx > 0 {
			startWidth = chars[startIdx-1].cumulativeWidth
		}

		remainingWidth := chars[len(chars)-1].cumulativeWidth - startWidth
		if remainingWidth <= maxWidth {
			line := text[chars[startIdx].byteOffset:]
			line = strings.TrimRight(line, " \t")

			if line != "" {
				lines = append(lines, line)
			}

			break
		}

		breakIdx := findBreakPointFromTable(chars, startIdx, maxWidth, startWidth)
		if breakIdx <= startIdx {
			breakIdx = startIdx + 1
		}

		lineStart := chars[startIdx].byteOffset

		lineEnd := len(text)
		if breakIdx < len(chars) {
			lineEnd = chars[breakIdx].byteOffset
		}

		line := text[lineStart:lineEnd]

		line = strings.TrimRight(line, " \t")
		if line != "" {
			lines = append(lines, line)
		}

		startIdx = skipLeadingSpaces(chars, breakIdx)
	}

	return lines
}

// skipLeadingSpaces finds the next non-space character index
func skipLeadingSpaces(chars []charInfo, startIdx int) int {
	for i := startIdx; i < len(chars); i++ {
		if !chars[i].isSpace {
			return i
		}
	}

	return len(chars)
}

// findBreakPointFromTable finds where to break the line using the char table
func findBreakPointFromTable(chars []charInfo, startIdx, maxWidth, startWidth int) int {
	if startIdx >= len(chars) {
		return len(chars)
	}

	lastSpaceIdx := -1
	lastSpaceWidth := 0

	for i := startIdx; i < len(chars); i++ {
		charWidth := chars[i].cumulativeWidth - startWidth

		if charWidth > maxWidth {
			if lastSpaceIdx >= 0 {
				wastedSpace := maxWidth - lastSpaceWidth
				if wastedSpace <= maxWastedSpace {
					return lastSpaceIdx + 1
				}
			}

			return i
		}

		if chars[i].isSpace {
			lastSpaceIdx = i
			lastSpaceWidth = charWidth
		}
	}

	return len(chars)
}

// buildCharTable creates a table mapping character positions to byte offsets and display widths
func buildCharTable(text string) []charInfo {
	if len(text) == 0 {
		return nil
	}

	var chars []charInfo

	bytePos := 0
	cumulativeWidth := 0

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
				byteOffset:      bytePos,
				width:           0,
				isSpace:         false,
				cumulativeWidth: cumulativeWidth,
			})

			bytePos += seqLen

			continue
		}

		r, size := utf8.DecodeRuneInString(text[bytePos:])
		if r == utf8.RuneError {
			cumulativeWidth++

			chars = append(chars, charInfo{
				byteOffset:      bytePos,
				width:           1,
				isSpace:         false,
				cumulativeWidth: cumulativeWidth,
			})
			bytePos++

			continue
		}

		width := ansi.PrintableRuneWidth(string(r))
		isSpace := r == ' ' || r == '\t'
		cumulativeWidth += width

		chars = append(chars, charInfo{
			byteOffset:      bytePos,
			width:           width,
			isSpace:         isSpace,
			cumulativeWidth: cumulativeWidth,
		})

		bytePos += size
	}

	return chars
}
