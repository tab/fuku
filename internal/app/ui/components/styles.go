package components

import "github.com/charmbracelet/lipgloss"

// Common styles shared across UI components
var (
	// HelpStyle for help text
	HelpStyle = lipgloss.NewStyle().
			Foreground(FgBorder)

	// TimestampStyle for timestamp text
	TimestampStyle = lipgloss.NewStyle().
			Foreground(FgMuted)

	// ErrorStyle for error messages
	ErrorStyle = lipgloss.NewStyle().
			Foreground(FgStatusError)

	// EmptyStateStyle for empty state messages
	EmptyStateStyle = lipgloss.NewStyle().
			Foreground(FgMuted).
			Padding(0, 1)

	// SpinnerStyle for loading spinners
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(FgPrimary)

	// Layout styles
	SeparatorStyle = lipgloss.NewStyle().
			Foreground(FgPrimary)

	HeaderStyle = lipgloss.NewStyle().
			MarginTop(1).
			MarginBottom(1)

	FooterStyle = lipgloss.NewStyle().
			MarginTop(1)

	FooterHelpStyle = lipgloss.NewStyle().
			MarginTop(1).
			Padding(0, 1)

	ContentStyle = lipgloss.NewStyle()

	// Services view styles
	HeaderTitleStyle = lipgloss.NewStyle().
				Foreground(FgPrimary).
				Bold(true)

	TierHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(FgPrimary).
			Padding(0, 1)

	ServiceRowStyle = lipgloss.NewStyle().
			Padding(0, 2)

	SelectedServiceRowStyle = lipgloss.NewStyle().
				Background(BgSelection).
				Padding(0, 2)

	StatusReadyStyle = lipgloss.NewStyle().
				Foreground(FgStatusReady).
				Bold(true)

	StatusStartingStyle = lipgloss.NewStyle().
				Foreground(FgStatusWarning).
				Bold(true)

	StatusFailedStyle = lipgloss.NewStyle().
				Foreground(FgStatusError).
				Bold(true)

	StatusStoppedStyle = lipgloss.NewStyle().
				Foreground(FgStatusStopped)

	PhaseStartingStyle = lipgloss.NewStyle().
				Foreground(FgStatusWarning)

	PhaseRunningStyle = lipgloss.NewStyle().
				Foreground(FgStatusReady)

	PhaseStoppingStyle = lipgloss.NewStyle().
				Foreground(FgStatusError)

	PhaseMutedStyle = lipgloss.NewStyle().
			Foreground(FgMuted)

	// Service indicator styles
	IndicatorActiveStyle = lipgloss.NewStyle().
				Foreground(FgStatusWarning)

	// Checkbox styles
	CheckboxCheckedStyle = lipgloss.NewStyle().
				Foreground(FgStatusReady)

	// Default/unknown status style
	StatusUnknownStyle = lipgloss.NewStyle()

	// Logs view styles
	ServiceNameStyle = lipgloss.NewStyle().
				Foreground(FgPrimary).
				Bold(true)

	// Log level styles
	LogLevelDebugStyle = lipgloss.NewStyle().
				Foreground(FgLogDebug)

	LogLevelInfoStyle = lipgloss.NewStyle().
				Foreground(FgLogInfo)

	LogLevelWarnStyle = lipgloss.NewStyle().
				Foreground(FgLogWarn)

	LogLevelErrorStyle = lipgloss.NewStyle().
				Foreground(FgLogError)

	// UUID style
	UUIDStyle = lipgloss.NewStyle().
			Foreground(FgLight)
)
