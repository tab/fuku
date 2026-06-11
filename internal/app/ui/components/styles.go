package components

import "charm.land/lipgloss/v2"

// FgPrimary is a fixed color (theme-independent, same in light and dark)
var FgPrimary = lipgloss.Color("#7D56F4")

// App layout styles
var (
	AppContainerStyle     = lipgloss.NewStyle().MarginTop(1)
	FooterStyle           = lipgloss.NewStyle().PaddingLeft(1)
	FooterMarginStyle     = lipgloss.NewStyle().MarginTop(1)
	TipStyle              = lipgloss.NewStyle().PaddingRight(1)
	ContentTopMarginStyle = lipgloss.NewStyle().MarginTop(1)
)

// Panel styles
var (
	PanelBorderStyle      = lipgloss.NewStyle().Foreground(FgPrimary)
	PanelMutedBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	PanelTitleStyle       = lipgloss.NewStyle().Foreground(FgPrimary).Bold(true)
	PanelTitleSpacer      = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
)

// Service list styles
var (
	TierContainerStyle = lipgloss.NewStyle().MarginBottom(1)
	TierHeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(FgPrimary).Padding(0, 1)
	ServiceRowStyle    = lipgloss.NewStyle().Padding(0, 2)
)

// Text styles
var (
	BoldStyle         = lipgloss.NewStyle().Bold(true)
	SpinnerStyle      = lipgloss.NewStyle().Foreground(FgPrimary)
	LoaderSpacerStyle = lipgloss.NewStyle().PaddingLeft(1)
)

// Doctor glyph styles
var (
	DoctorGlyphOKStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	DoctorGlyphIdleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	DoctorGlyphNoteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	DoctorGlyphWarnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	DoctorGlyphFailStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)
