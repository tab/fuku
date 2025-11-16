package services

import (
	"github.com/charmbracelet/lipgloss"

	"fuku/internal/app/ui/components"
)

var (
	titleStyle      = components.TitleStyle
	statusStyle     = components.StatusStyle
	helpStyle       = components.HelpStyle
	timestampStyle  = components.TimestampStyle
	errorStyle      = components.ErrorStyle
	emptyStateStyle = components.EmptyStateStyle
	spinnerStyle    = components.SpinnerStyle

	activePanelStyle = components.PanelStyle

	tierHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(components.ColorPrimary)

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
)
