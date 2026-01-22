package components

import "github.com/charmbracelet/lipgloss"

// App layout styles
var (
	AppContainerStyle = lipgloss.NewStyle().MarginTop(1)
	FooterStyle       = lipgloss.NewStyle().PaddingLeft(1)
	FooterMarginStyle = lipgloss.NewStyle().MarginTop(1)
	TipStyle          = lipgloss.NewStyle().PaddingRight(1)
)

// Panel styles
var (
	PanelBorderStyle = lipgloss.NewStyle().Foreground(FgPrimary)
	PanelMutedStyle  = lipgloss.NewStyle().Foreground(FgMuted)
	PanelTitleStyle  = lipgloss.NewStyle().Foreground(FgPrimary).Bold(true)
	PanelTitleSpacer = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
)

// Service list styles
var (
	TierContainerStyle      = lipgloss.NewStyle().MarginBottom(1)
	TierHeaderStyle         = lipgloss.NewStyle().Bold(true).Foreground(FgPrimary).Padding(0, 1)
	ServiceHeaderStyle      = lipgloss.NewStyle().Foreground(FgMuted).Padding(0, 2)
	ServiceRowStyle         = lipgloss.NewStyle().Padding(0, 2)
	SelectedServiceRowStyle = lipgloss.NewStyle().Background(BgSelection).Padding(0, 2)
)

// Status styles
var (
	StatusRunningStyle  = lipgloss.NewStyle().Foreground(FgStatusRunning).Bold(true)
	StatusStartingStyle = lipgloss.NewStyle().Foreground(FgStatusWarning).Bold(true)
	StatusFailedStyle   = lipgloss.NewStyle().Foreground(FgStatusError).Bold(true)
	StatusStoppedStyle  = lipgloss.NewStyle().Foreground(FgStatusStopped).Bold(true)
)

// Phase styles
var (
	PhaseStartingStyle = lipgloss.NewStyle().Foreground(FgStatusWarning)
	PhaseRunningStyle  = lipgloss.NewStyle().Foreground(FgStatusRunning)
	PhaseStoppingStyle = lipgloss.NewStyle().Foreground(FgStatusError)
	PhaseMutedStyle    = lipgloss.NewStyle().Foreground(FgMuted)
)

// Common styles
var (
	HelpStyle            = lipgloss.NewStyle().Foreground(FgBorder)
	ErrorStyle           = lipgloss.NewStyle().Foreground(FgStatusError)
	EmptyStateStyle      = lipgloss.NewStyle().Foreground(FgMuted).Padding(0, 1)
	SpinnerStyle         = lipgloss.NewStyle().Foreground(FgPrimary)
	IndicatorActiveStyle = lipgloss.NewStyle().Foreground(FgStatusWarning)
	LoaderSpacerStyle    = lipgloss.NewStyle().PaddingLeft(1)
)
