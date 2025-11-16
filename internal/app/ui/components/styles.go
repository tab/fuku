package components

import "github.com/charmbracelet/lipgloss"

// Common styles shared across UI components
var (
	// TitleStyle for view titles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Padding(1, 2, 0, 2)

	// StatusStyle for status information
	StatusStyle = lipgloss.NewStyle().
			Padding(1, 2, 0, 0)

	// PanelStyle for active panel borders
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1)

	// HelpStyle for help text
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorBorder).
			Padding(0, 2)

	// TimestampStyle for timestamp text
	TimestampStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// ErrorStyle for error messages
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorFailed)

	// EmptyStateStyle for empty state messages
	EmptyStateStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(2)

	// SpinnerStyle for loading spinners
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)
)
