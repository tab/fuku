package logs

import "github.com/charmbracelet/lipgloss"

const (
	colorPrimary = lipgloss.Color("#7D56F4")
	colorMuted   = lipgloss.Color("7")
)

var (
	serviceNameStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	timestampStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	logMessageStyle = lipgloss.NewStyle()

	emptyStateStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(2)
)
