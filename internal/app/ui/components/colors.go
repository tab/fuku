package components

import "github.com/charmbracelet/lipgloss"

// Color palette for the UI
const (
	ColorPrimary  = lipgloss.Color("#7D56F4") // Purple - primary/focus color
	ColorBorder   = lipgloss.Color("8")       // Gray - borders and help text
	ColorMuted    = lipgloss.Color("7")       // Light gray - muted elements
	ColorReady    = lipgloss.Color("10")      // Green - ready status
	ColorStarting = lipgloss.Color("11")      // Yellow - starting status
	ColorFailed   = lipgloss.Color("9")       // Red - failed status
	ColorStopped  = lipgloss.Color("8")       // Gray - stopped status
	ColorSelected = lipgloss.Color("235")     // Dark gray - selected background
)
