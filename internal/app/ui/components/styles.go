package components

import "github.com/charmbracelet/lipgloss"

// Common styles shared across UI components
var (
	HelpStyle = lipgloss.NewStyle().
			Foreground(FgBorder)

	TimestampStyle = lipgloss.NewStyle().
			Foreground(FgMuted)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(FgStatusError)

	EmptyStateStyle = lipgloss.NewStyle().
			Foreground(FgMuted).
			Padding(0, 1)

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(FgPrimary)

	SeparatorStyle = lipgloss.NewStyle().
			Foreground(FgPrimary)

	HeaderStyle = lipgloss.NewStyle().
			MarginTop(1).
			MarginBottom(1)
	HeaderTitleStyle = lipgloss.NewStyle().
				Foreground(FgPrimary).
				Bold(true)

	FooterStyle = lipgloss.NewStyle().
			MarginTop(1)
	FooterHelpStyle = lipgloss.NewStyle().
			MarginTop(1).
			Padding(0, 1)

	TierContainerStyle = lipgloss.NewStyle().
				MarginBottom(1)
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
				Foreground(FgStatusStopped).
				Bold(true)

	PhaseStartingStyle = lipgloss.NewStyle().
				Foreground(FgStatusWarning)
	PhaseRunningStyle = lipgloss.NewStyle().
				Foreground(FgStatusReady)
	PhaseStoppingStyle = lipgloss.NewStyle().
				Foreground(FgStatusError)
	PhaseMutedStyle = lipgloss.NewStyle().
			Foreground(FgMuted)

	IndicatorActiveStyle = lipgloss.NewStyle().
				Foreground(FgStatusWarning)

	ServiceNameStyle = lipgloss.NewStyle().
				Foreground(FgPrimary).
				Bold(true)

	LogLevelDebugStyle = lipgloss.NewStyle().
				Foreground(FgLogDebug)
	LogLevelInfoStyle = lipgloss.NewStyle().
				Foreground(FgLogInfo)
	LogLevelWarnStyle = lipgloss.NewStyle().
				Foreground(FgLogWarn)
	LogLevelErrorStyle = lipgloss.NewStyle().
				Foreground(FgLogError)

	UUIDStyle = lipgloss.NewStyle().
			Foreground(FgLight)
)
