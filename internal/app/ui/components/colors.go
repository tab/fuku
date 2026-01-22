package components

import "github.com/charmbracelet/lipgloss"

// Color palette for the UI with semantic naming
const (
	// Foreground colors - text and elements
	FgPrimary = lipgloss.Color("#7D56F4") // Purple - primary/focus color
	FgMuted   = lipgloss.Color("7")       // Light gray - muted elements
	FgBorder  = lipgloss.Color("8")       // Gray - borders and help text

	// Background colors
	BgSelection = lipgloss.Color("235") // Dark gray - selected background

	// Status colors - service states
	FgStatusRunning = lipgloss.Color("10") // Green - running status
	FgStatusWarning = lipgloss.Color("11") // Yellow - starting/warning status
	FgStatusError   = lipgloss.Color("9")  // Red - failed/error status
	FgStatusStopped = lipgloss.Color("8")  // Gray - stopped status
)

// LogSeparatorColor is the adaptive color for log separators
var LogSeparatorColor = lipgloss.AdaptiveColor{Light: "#737373", Dark: "#a3a3a3"}

// ServiceColorPalette provides distinct colors for service names (24 colors)
var ServiceColorPalette = []lipgloss.AdaptiveColor{
	{Light: "#0891b2", Dark: "#22d3ee"}, // Cyan
	{Light: "#d97706", Dark: "#fbbf24"}, // Amber
	{Light: "#059669", Dark: "#34d399"}, // Emerald
	{Light: "#7c3aed", Dark: "#a78bfa"}, // Violet
	{Light: "#db2777", Dark: "#f472b6"}, // Pink
	{Light: "#2563eb", Dark: "#60a5fa"}, // Blue
	{Light: "#dc2626", Dark: "#f87171"}, // Red
	{Light: "#65a30d", Dark: "#a3e635"}, // Lime
	{Light: "#0d9488", Dark: "#2dd4bf"}, // Teal
	{Light: "#ea580c", Dark: "#fb923c"}, // Orange
	{Light: "#4f46e5", Dark: "#818cf8"}, // Indigo
	{Light: "#be185d", Dark: "#f472b6"}, // Fuchsia
	{Light: "#0284c7", Dark: "#38bdf8"}, // Sky
	{Light: "#b91c1c", Dark: "#fca5a5"}, // Rose
	{Light: "#15803d", Dark: "#86efac"}, // Green
	{Light: "#6d28d9", Dark: "#c4b5fd"}, // Purple
	{Light: "#c2410c", Dark: "#fdba74"}, // Burnt Orange
	{Light: "#0e7490", Dark: "#67e8f9"}, // Cyan Dark
	{Light: "#7e22ce", Dark: "#d8b4fe"}, // Purple Light
	{Light: "#166534", Dark: "#4ade80"}, // Green Dark
	{Light: "#9333ea", Dark: "#e879f9"}, // Magenta
	{Light: "#1d4ed8", Dark: "#93c5fd"}, // Blue Light
	{Light: "#b45309", Dark: "#fcd34d"}, // Gold
	{Light: "#047857", Dark: "#6ee7b7"}, // Mint
}
