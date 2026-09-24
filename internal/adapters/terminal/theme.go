package terminal

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

// Theme holds colors and pre-built styles that differ between light and dark terminals
type Theme struct {
	Appearance          Appearance
	ServiceColorPalette []color.Color

	// TUI styles (services screen)
	PanelMutedStyle       lipgloss.Style
	PanelMutedBorderStyle lipgloss.Style
	PlaceholderStyle      lipgloss.Style
	CurrentVersionStyle   lipgloss.Style
	LatestVersionStyle    lipgloss.Style
	ServiceHeaderStyle    lipgloss.Style
	SelectedRowStyle      lipgloss.Style
	SelectionBgStyle      lipgloss.Style
	StatusPendingStyle    lipgloss.Style
	StatusRunningStyle    lipgloss.Style
	StatusStartingStyle   lipgloss.Style
	StatusFailedStyle     lipgloss.Style
	StatusStoppedStyle    lipgloss.Style
	PhaseStartingStyle    lipgloss.Style
	PhaseRunningStyle     lipgloss.Style
	PhaseStoppingStyle    lipgloss.Style
	PhaseMutedStyle       lipgloss.Style
	HelpStyle             lipgloss.Style
	ErrorStyle            lipgloss.Style
	EmptyStateStyle       lipgloss.Style
	IndicatorActiveStyle  lipgloss.Style
	IndicatorDotStyle     lipgloss.Style
	APIDotConnected       lipgloss.Style
	APIDotDisconnected    lipgloss.Style

	// Aside styles
	AsideSectionTitleStyle lipgloss.Style

	// Timeline strip styles
	TimelineRunningStyle  lipgloss.Style
	TimelineStartingStyle lipgloss.Style
	TimelineFailedStyle   lipgloss.Style
	TimelineStoppedStyle  lipgloss.Style
	TimelineEmptyStyle    lipgloss.Style

	// Timeline strip styles (selected row)
	TimelineSelectedRunningStyle  lipgloss.Style
	TimelineSelectedStartingStyle lipgloss.Style
	TimelineSelectedFailedStyle   lipgloss.Style
	TimelineSelectedStoppedStyle  lipgloss.Style
	TimelineSelectedEmptyStyle    lipgloss.Style

	// Shared styles (TUI tips + logs banner)
	HelpKeyStyle  lipgloss.Style
	HelpDescStyle lipgloss.Style

	// Logs styles (logs screen)
	LogsSeparatorStyle lipgloss.Style

	// Doctor glyph styles
	DoctorGlyphOKStyle   lipgloss.Style
	DoctorGlyphIdleStyle lipgloss.Style
	DoctorGlyphNoteStyle lipgloss.Style
	DoctorGlyphWarnStyle lipgloss.Style
	DoctorGlyphFailStyle lipgloss.Style
}

// Appearance is the terminal background the theme adapts to
type Appearance string

// Appearances a theme is built for; system resolves to light or dark from the terminal
const (
	AppearanceLight  Appearance = "light"
	AppearanceDark   Appearance = "dark"
	AppearanceSystem Appearance = "system"
)

// Resolve returns dark or light from the terminal background for system, and the appearance itself otherwise
func (a Appearance) Resolve(in term.File, out term.File) Appearance {
	if a != AppearanceSystem {
		return a
	}

	if lipgloss.HasDarkBackground(in, out) {
		return AppearanceDark
	}

	return AppearanceLight
}

// NewTheme creates a theme for a light or dark appearance and never detects the terminal background
func NewTheme(appearance Appearance) Theme {
	ld := lipgloss.LightDark(appearance == AppearanceDark)

	fgMuted := ld(lipgloss.Color("#737373"), lipgloss.Color("7"))
	fgPlaceholder := ld(lipgloss.Color("#737373"), lipgloss.Color("#a3a3a3"))
	fgBorder := ld(lipgloss.Color("#a3a3a3"), lipgloss.Color("8"))
	fgStatusRunning := ld(lipgloss.Color("#16a34a"), lipgloss.Color("10"))
	fgStatusWarning := ld(lipgloss.Color("#d97706"), lipgloss.Color("11"))
	fgStatusError := ld(lipgloss.Color("#dc2626"), lipgloss.Color("9"))
	fgStatusNote := ld(lipgloss.Color("#0891b2"), lipgloss.Color("14"))
	fgStatusStopped := fgBorder
	bgSelection := ld(lipgloss.Color("#d4d4d4"), lipgloss.Color("235"))

	return Theme{
		Appearance:          appearance,
		ServiceColorPalette: buildServicePalette(ld),

		PanelMutedStyle:       lipgloss.NewStyle().Foreground(fgMuted),
		PanelMutedBorderStyle: lipgloss.NewStyle().Foreground(fgBorder),
		PlaceholderStyle:      lipgloss.NewStyle().Foreground(fgPlaceholder).Padding(0, 1),
		CurrentVersionStyle:   lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#171717"), lipgloss.Color("15"))),
		LatestVersionStyle:    lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#dc6543"), lipgloss.Color("#FF7F50"))),
		ServiceHeaderStyle:    lipgloss.NewStyle().Foreground(fgMuted).Padding(0, 2),
		SelectedRowStyle:      lipgloss.NewStyle().Background(bgSelection).Padding(0, 2),
		SelectionBgStyle:      lipgloss.NewStyle().Background(bgSelection),
		StatusPendingStyle:    lipgloss.NewStyle().Foreground(fgMuted),
		StatusRunningStyle:    lipgloss.NewStyle().Foreground(fgStatusRunning),
		StatusStartingStyle:   lipgloss.NewStyle().Foreground(fgStatusWarning),
		StatusFailedStyle:     lipgloss.NewStyle().Foreground(fgStatusError),
		StatusStoppedStyle:    lipgloss.NewStyle().Foreground(fgStatusStopped),
		PhaseStartingStyle:    lipgloss.NewStyle().Foreground(fgStatusWarning),
		PhaseRunningStyle:     lipgloss.NewStyle().Foreground(fgStatusRunning),
		PhaseStoppingStyle:    lipgloss.NewStyle().Foreground(fgStatusError),
		PhaseMutedStyle:       lipgloss.NewStyle().Foreground(fgMuted),
		HelpStyle:             lipgloss.NewStyle().Foreground(fgBorder),
		ErrorStyle:            lipgloss.NewStyle().Foreground(fgStatusError),
		EmptyStateStyle:       lipgloss.NewStyle().Foreground(fgMuted).Padding(0, 1),
		IndicatorActiveStyle:  lipgloss.NewStyle().Foreground(fgStatusWarning),
		IndicatorDotStyle:     lipgloss.NewStyle().Foreground(fgStatusRunning),
		APIDotConnected:       lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#0284c7"), lipgloss.Color("#38bdf8"))),
		APIDotDisconnected:    lipgloss.NewStyle().Foreground(fgBorder),

		AsideSectionTitleStyle: lipgloss.NewStyle().Foreground(fgMuted).Bold(true),

		TimelineRunningStyle:  lipgloss.NewStyle().Foreground(fgStatusRunning),
		TimelineStartingStyle: lipgloss.NewStyle().Foreground(fgStatusWarning),
		TimelineFailedStyle:   lipgloss.NewStyle().Foreground(fgStatusError),
		TimelineStoppedStyle:  lipgloss.NewStyle().Foreground(fgBorder),
		TimelineEmptyStyle:    lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#b8b8b8"), lipgloss.Color("#4a4a4a"))),

		TimelineSelectedRunningStyle:  lipgloss.NewStyle().Foreground(fgStatusRunning).Background(bgSelection),
		TimelineSelectedStartingStyle: lipgloss.NewStyle().Foreground(fgStatusWarning).Background(bgSelection),
		TimelineSelectedFailedStyle:   lipgloss.NewStyle().Foreground(fgStatusError).Background(bgSelection),
		TimelineSelectedStoppedStyle:  lipgloss.NewStyle().Foreground(fgBorder).Background(bgSelection),
		TimelineSelectedEmptyStyle:    lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#b8b8b8"), lipgloss.Color("#4a4a4a"))).Background(bgSelection),

		HelpKeyStyle:  lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#909090"), lipgloss.Color("#626262"))),
		HelpDescStyle: lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#B2B2B2"), lipgloss.Color("#4A4A4A"))),

		LogsSeparatorStyle: lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#737373"), lipgloss.Color("#a3a3a3"))),

		DoctorGlyphOKStyle:   lipgloss.NewStyle().Foreground(fgStatusRunning),
		DoctorGlyphIdleStyle: lipgloss.NewStyle().Foreground(fgBorder),
		DoctorGlyphNoteStyle: lipgloss.NewStyle().Foreground(fgStatusNote),
		DoctorGlyphWarnStyle: lipgloss.NewStyle().Foreground(fgStatusWarning),
		DoctorGlyphFailStyle: lipgloss.NewStyle().Foreground(fgStatusError),
	}
}

// newLogsServiceNameStyle creates a bold style for a service name color
func (t Theme) newLogsServiceNameStyle(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c).Bold(true)
}

// buildServicePalette creates the 24-color service name palette for the given mode
func buildServicePalette(ld lipgloss.LightDarkFunc) []color.Color {
	type pair struct{ light, dark string }

	pairs := []pair{
		{"#0891b2", "#22d3ee"},
		{"#d97706", "#fbbf24"},
		{"#059669", "#34d399"},
		{"#7c3aed", "#a78bfa"},
		{"#db2777", "#f472b6"},
		{"#2563eb", "#60a5fa"},
		{"#dc2626", "#f87171"},
		{"#65a30d", "#a3e635"},
		{"#0d9488", "#2dd4bf"},
		{"#ea580c", "#fb923c"},
		{"#4f46e5", "#818cf8"},
		{"#c026d3", "#e879f9"},
		{"#0284c7", "#38bdf8"},
		{"#e11d48", "#fb7185"},
		{"#16a34a", "#4ade80"},
		{"#9333ea", "#c084fc"},
		{"#ca8a04", "#facc15"},
		{"#0e7490", "#67e8f9"},
		{"#6d28d9", "#c4b5fd"},
		{"#047857", "#6ee7b7"},
		{"#be185d", "#f9a8d4"},
		{"#1d4ed8", "#93c5fd"},
		{"#b45309", "#fcd34d"},
		{"#0f766e", "#5eead4"},
	}

	palette := make([]color.Color, len(pairs))
	for i, p := range pairs {
		palette[i] = ld(lipgloss.Color(p.light), lipgloss.Color(p.dark))
	}

	return palette
}
