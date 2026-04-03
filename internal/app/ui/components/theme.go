package components

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme holds colors and pre-built styles that differ between light and dark terminals
type Theme struct {
	IsDark              bool
	ServiceColorPalette []color.Color

	// Semantic colors
	FgMuted         color.Color
	FgBorder        color.Color
	FgStatusRunning color.Color
	FgStatusWarning color.Color
	FgStatusError   color.Color
	FgStatusStopped color.Color
	BgSelection     color.Color

	// TUI styles (services screen)
	PanelMutedStyle      lipgloss.Style
	ServiceHeaderStyle   lipgloss.Style
	SelectedRowStyle     lipgloss.Style
	StatusRunningStyle   lipgloss.Style
	StatusStartingStyle  lipgloss.Style
	StatusFailedStyle    lipgloss.Style
	StatusStoppedStyle   lipgloss.Style
	PhaseStartingStyle   lipgloss.Style
	PhaseRunningStyle    lipgloss.Style
	PhaseStoppingStyle   lipgloss.Style
	PhaseMutedStyle      lipgloss.Style
	HelpStyle            lipgloss.Style
	ErrorStyle           lipgloss.Style
	EmptyStateStyle      lipgloss.Style
	IndicatorActiveStyle lipgloss.Style
	IndicatorDotStyle    lipgloss.Style
	APIDotConnected      lipgloss.Style
	APIDotDisconnected   lipgloss.Style

	// Shared styles (TUI tips + logs banner)
	HelpKeyStyle  lipgloss.Style
	HelpDescStyle lipgloss.Style

	// Logs styles (logs screen)
	LogsSeparatorStyle lipgloss.Style
}

// NewTheme creates a theme for the given background mode
func NewTheme(isDark bool) Theme {
	ld := lipgloss.LightDark(isDark)

	fgMuted := ld(lipgloss.Color("#737373"), lipgloss.Color("7"))
	fgBorder := ld(lipgloss.Color("#a3a3a3"), lipgloss.Color("8"))
	fgStatusRunning := ld(lipgloss.Color("#16a34a"), lipgloss.Color("10"))
	fgStatusWarning := ld(lipgloss.Color("#d97706"), lipgloss.Color("11"))
	fgStatusError := ld(lipgloss.Color("#dc2626"), lipgloss.Color("9"))
	fgStatusStopped := fgBorder
	bgSelection := ld(lipgloss.Color("#d4d4d4"), lipgloss.Color("235"))

	return Theme{
		IsDark:              isDark,
		ServiceColorPalette: buildServicePalette(ld),

		FgMuted:         fgMuted,
		FgBorder:        fgBorder,
		FgStatusRunning: fgStatusRunning,
		FgStatusWarning: fgStatusWarning,
		FgStatusError:   fgStatusError,
		FgStatusStopped: fgStatusStopped,
		BgSelection:     bgSelection,

		LogsSeparatorStyle:   lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#737373"), lipgloss.Color("#a3a3a3"))),
		HelpKeyStyle:         lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#909090"), lipgloss.Color("#626262"))),
		HelpDescStyle:        lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#B2B2B2"), lipgloss.Color("#4A4A4A"))),
		PanelMutedStyle:      lipgloss.NewStyle().Foreground(fgMuted),
		ServiceHeaderStyle:   lipgloss.NewStyle().Foreground(fgMuted).Padding(0, 2),
		SelectedRowStyle:     lipgloss.NewStyle().Background(bgSelection).Padding(0, 2),
		StatusRunningStyle:   lipgloss.NewStyle().Foreground(fgStatusRunning),
		StatusStartingStyle:  lipgloss.NewStyle().Foreground(fgStatusWarning),
		StatusFailedStyle:    lipgloss.NewStyle().Foreground(fgStatusError),
		StatusStoppedStyle:   lipgloss.NewStyle().Foreground(fgStatusStopped),
		PhaseStartingStyle:   lipgloss.NewStyle().Foreground(fgStatusWarning),
		PhaseRunningStyle:    lipgloss.NewStyle().Foreground(fgStatusRunning),
		PhaseStoppingStyle:   lipgloss.NewStyle().Foreground(fgStatusError),
		PhaseMutedStyle:      lipgloss.NewStyle().Foreground(fgMuted),
		HelpStyle:            lipgloss.NewStyle().Foreground(fgBorder),
		ErrorStyle:           lipgloss.NewStyle().Foreground(fgStatusError),
		EmptyStateStyle:      lipgloss.NewStyle().Foreground(fgMuted).Padding(0, 1),
		IndicatorActiveStyle: lipgloss.NewStyle().Foreground(fgStatusWarning),
		IndicatorDotStyle:    lipgloss.NewStyle().Foreground(fgStatusRunning),
		APIDotConnected:      lipgloss.NewStyle().Foreground(fgStatusRunning),
		APIDotDisconnected:   lipgloss.NewStyle().Foreground(fgBorder),
	}
}

// DefaultTheme returns a theme assuming dark background
func DefaultTheme() Theme {
	return NewTheme(true)
}

// NewLogsServiceNameStyle creates a bold style for a service name color
func (t Theme) NewLogsServiceNameStyle(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c).Bold(true)
}

// buildServicePalette creates the 24-color service name palette for the given mode
func buildServicePalette(ld lipgloss.LightDarkFunc) []color.Color {
	type pair struct{ light, dark string }

	pairs := []pair{
		{"#0891b2", "#22d3ee"}, // Cyan
		{"#d97706", "#fbbf24"}, // Amber
		{"#059669", "#34d399"}, // Emerald
		{"#7c3aed", "#a78bfa"}, // Violet
		{"#db2777", "#f472b6"}, // Pink
		{"#2563eb", "#60a5fa"}, // Blue
		{"#dc2626", "#f87171"}, // Red
		{"#65a30d", "#a3e635"}, // Lime
		{"#0d9488", "#2dd4bf"}, // Teal
		{"#ea580c", "#fb923c"}, // Orange
		{"#4f46e5", "#818cf8"}, // Indigo
		{"#c026d3", "#e879f9"}, // Fuchsia
		{"#0284c7", "#38bdf8"}, // Sky
		{"#e11d48", "#fb7185"}, // Rose
		{"#16a34a", "#4ade80"}, // Green
		{"#9333ea", "#c084fc"}, // Purple
		{"#ca8a04", "#facc15"}, // Yellow
		{"#0e7490", "#67e8f9"}, // Deep Cyan
		{"#6d28d9", "#c4b5fd"}, // Deep Violet
		{"#047857", "#6ee7b7"}, // Deep Emerald
		{"#be185d", "#f9a8d4"}, // Deep Pink
		{"#1d4ed8", "#93c5fd"}, // Deep Blue
		{"#b45309", "#fcd34d"}, // Gold
		{"#0f766e", "#5eead4"}, // Mint
	}

	palette := make([]color.Color, len(pairs))
	for i, p := range pairs {
		palette[i] = ld(lipgloss.Color(p.light), lipgloss.Color(p.dark))
	}

	return palette
}
