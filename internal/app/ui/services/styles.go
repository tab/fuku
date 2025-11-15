package services

import "github.com/charmbracelet/lipgloss"

const (
	ColorPrimary  = lipgloss.Color("#7D56F4") // Purple - primary/focus color
	ColorBorder   = lipgloss.Color("8")       // Gray - borders and help text
	ColorMuted    = lipgloss.Color("7")       // Light gray - muted elements
	ColorReady    = lipgloss.Color("10")      // Green - ready status
	ColorStarting = lipgloss.Color("11")      // Yellow - starting status
	ColorFailed   = lipgloss.Color("9")       // Red - failed status
	ColorStopped  = lipgloss.Color("8")       // Gray - stopped status
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Padding(1, 2, 0, 2)

	statusStyle = lipgloss.NewStyle().
			Padding(1, 2, 0, 0)

	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(ColorBorder).
			Padding(0, 2)

	tierHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginTop(1).
			MarginBottom(0)

	serviceRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	selectedServiceRowStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("235"))

	statusReadyStyle = lipgloss.NewStyle().
				Foreground(ColorReady).
				Bold(true)

	statusStartingStyle = lipgloss.NewStyle().
				Foreground(ColorStarting).
				Bold(true)

	statusFailedStyle = lipgloss.NewStyle().
				Foreground(ColorFailed).
				Bold(true)

	statusStoppedStyle = lipgloss.NewStyle().
				Foreground(ColorStopped)

	timestampStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	serviceNameStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true)

	streamStdoutStyle = lipgloss.NewStyle().
				Foreground(ColorReady)

	streamStderrStyle = lipgloss.NewStyle().
				Foreground(ColorFailed)

	errorStyle = lipgloss.NewStyle().
			Foreground(ColorFailed)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)
)
