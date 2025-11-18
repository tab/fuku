package services

import (
	"github.com/charmbracelet/lipgloss"

	"fuku/internal/app/ui/components"
)

var (
	helpStyle       = components.HelpStyle
	timestampStyle  = components.TimestampStyle
	errorStyle      = components.ErrorStyle
	emptyStateStyle = components.EmptyStateStyle
	spinnerStyle    = components.SpinnerStyle

	borderCharStyle = lipgloss.NewStyle().
			Foreground(components.ColorPrimary)

	headerStyle = lipgloss.NewStyle().
			MarginTop(1)

	helpWrapperStyle = lipgloss.NewStyle().
				MarginTop(1)

	headerTitleStyle = lipgloss.NewStyle().
				Foreground(components.ColorPrimary).
				Bold(true)

	tierHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(components.ColorPrimary).
			Margin(1, 0, 0, 0)

	serviceRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	selectedServiceRowStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(components.ColorSelected)

	statusReadyStyle = lipgloss.NewStyle().
				Foreground(components.ColorReady).
				Bold(true)

	statusStartingStyle = lipgloss.NewStyle().
				Foreground(components.ColorStarting).
				Bold(true)

	statusFailedStyle = lipgloss.NewStyle().
				Foreground(components.ColorFailed).
				Bold(true)

	statusStoppedStyle = lipgloss.NewStyle().
				Foreground(components.ColorStopped)

	phaseStartingStyle = lipgloss.NewStyle().
				Foreground(components.ColorStarting)

	phaseRunningStyle = lipgloss.NewStyle().
				Foreground(components.ColorReady)

	phaseStoppingStyle = lipgloss.NewStyle().
				Foreground(components.ColorFailed)

	phaseMutedStyle = lipgloss.NewStyle().
			Foreground(components.ColorMuted)

	noBorderTopPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(components.ColorPrimary).
				BorderTop(false).
				Padding(0, 1)
)
