package cli

import (
	"fuku/internal/config"

	"github.com/charmbracelet/lipgloss"
)

// Material Design 3 Typography Scale
// https://m3.material.io/styles/typography/overview

// Display - Large, expressive text for short important content
var (
// displayLarge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).MarginTop(1).MarginBottom(1)
// displayMedium = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).MarginTop(1)
// displaySmall  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
)

// Headline - High-emphasis text for section headers
var (
	headlineLarge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).MarginTop(1)
	// headlineMedium = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9D7BF5"))
	// headlineSmall  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9D7BF5"))
)

// Title - Medium-emphasis text for titles and subtitles
var (
	// titleLarge  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B575"))
	titleMedium = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B575"))
	// titleSmall  = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
)

// Body - Main content text
var (
	bodyLarge  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
	bodyMedium = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
	// bodySmall  = lipgloss.NewStyle().Foreground(lipgloss.Color("#BDBDBD"))
)

// Label - Small text for labels, captions, and supplementary content
var (
	labelLarge = lipgloss.NewStyle().Foreground(lipgloss.Color("#9E9E9E")).Italic(true).MarginTop(2)
	// labelMedium = lipgloss.NewStyle().Foreground(lipgloss.Color("#9E9E9E")).Italic(true)
	// labelSmall  = lipgloss.NewStyle().Foreground(lipgloss.Color("#757575"))
)

// Semantic styles - mapped to Material typography scale
var (
	// Primary content
	sectionHeader = headlineLarge.MarginBottom(1)
	helpText      = labelLarge

	// Accents and highlights
	commandName = titleMedium
	exampleCode = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFA726"))

	// Title components (inline styles without margins)
	appNameStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	appVersionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#BDBDBD"))
	titleWrapper    = lipgloss.NewStyle().MarginTop(1).MarginBottom(1)

	// Log styles
	logTimestampStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	logServiceNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#2D5016")).Bold(true)
)

// RenderTitle renders the app title block with name, version, and description
func RenderTitle() string {
	title := titleWrapper.Render(
		appNameStyle.Render(config.AppName) + appVersionStyle.Render(" v"+config.Version),
	)
	description := bodyLarge.Render(config.AppDescription)

	return lipgloss.JoinVertical(lipgloss.Left, title, description)
}

func RenderHelp() string {
	return helpText.Render("Press q or esc to exit")
}
