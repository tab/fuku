package components

import "github.com/charmbracelet/lipgloss"

// Common styles shared across UI components
var (
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
			MarginTop(1)

	// SpinnerStyle for loading spinners
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)
)
