package components

import "github.com/charmbracelet/lipgloss"

// Color palette for the UI with semantic naming
const (
	// Foreground colors - text and elements
	FgPrimary = lipgloss.Color("#7D56F4") // Purple - primary/focus color
	FgMuted   = lipgloss.Color("7")       // Light gray - muted elements
	FgLight   = lipgloss.Color("15")      // White - light text
	FgBorder  = lipgloss.Color("8")       // Gray - borders and help text

	// Background colors
	BgSelection = lipgloss.Color("235") // Dark gray - selected background

	// Status colors - service states
	FgStatusRunning = lipgloss.Color("10") // Green - running status
	FgStatusWarning = lipgloss.Color("11") // Yellow - starting/warning status
	FgStatusError   = lipgloss.Color("9")  // Red - failed/error status
	FgStatusStopped = lipgloss.Color("8")  // Gray - stopped status

	// Log level colors
	FgLogDebug = lipgloss.Color("8")  // Gray - debug messages
	FgLogInfo  = lipgloss.Color("12") // Blue - info messages
	FgLogWarn  = lipgloss.Color("11") // Yellow - warning messages
	FgLogError = lipgloss.Color("9")  // Red - error messages
)
