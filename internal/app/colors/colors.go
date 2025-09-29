package colors

import (
	"fmt"
)

// ANSI color codes for terminal output
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
	Italic = "\033[3m"

	// Text colors
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"

	// Background colors
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgWhite   = "\033[47m"
)

// Color functions for semantic styling
func Primary(text string) string {
	return Magenta + text + Reset
}

func Success(text string) string {
	return Green + text + Reset
}

func Warning(text string) string {
	return Yellow + text + Reset
}

func Error(text string) string {
	return Red + text + Reset
}

func Info(text string) string {
	return Blue + text + Reset
}

func Muted(text string) string {
	return Gray + text + Reset
}

func Title(text string) string {
	return Bold + White + text + Reset
}

func Subtitle(text string) string {
	return Bold + text + Reset
}

// UI symbols for clean interface
const (
	// Status symbols
	StatusRunning = "⏺"
	StatusSuccess = "●"
	StatusFailed  = "●"
	StatusPending = "○"
	StatusStopped = "○"

	// Tree symbols
	TreeBranch = "├─"
	TreeLast   = "└─"
	TreePipe   = "│"
	TreeSpace  = "  "

	// Progress symbols
	ProgressBar   = "█"
	ProgressEmpty = "░"
	ProgressArrow = "⎿"

	// Phase symbols
	Phase = "⏺"
)

// Colorize status symbols
func StatusRunningColor(symbol string) string {
	return Green + symbol + Reset
}

func StatusSuccessColor(symbol string) string {
	return Green + symbol + Reset
}

func StatusFailedColor(symbol string) string {
	return Red + symbol + Reset
}

func StatusPendingColor(symbol string) string {
	return Gray + symbol + Reset
}

func StatusStoppedColor(symbol string) string {
	return Gray + symbol + Reset
}

// Phase colors
func PhaseColor(phase, symbol string) string {
	switch phase {
	case "Discovery":
		return White + symbol + Reset
	case "Planning":
		return Yellow + symbol + Reset
	case "Execution":
		return Green + symbol + Reset
	default:
		return Gray + symbol + Reset
	}
}

// Format phase with timing
func FormatPhase(phase, details string) string {
	symbol := PhaseColor(phase, Phase)
	return fmt.Sprintf("%s %s\n  %s %s", symbol, Title(phase), ProgressArrow, Muted(details))
}
