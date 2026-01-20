package components

import "github.com/charmbracelet/lipgloss"

// Tip styles
var (
	tipKeyStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#909090", Dark: "#626262"})
	tipDescStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B2B2B2", Dark: "#4A4A4A"})
)

func tipKey(k string) string  { return tipKeyStyle.Render(k) }
func tipDesc(d string) string { return tipDescStyle.Render(d) }

// Tips contains helpful hints displayed in the footer
var Tips = []string{
	tipDesc("Stream logs with ") + tipKey("fuku --logs"),
	tipDesc("Filter logs by service with ") + tipKey("fuku --logs api"),
	tipDesc("Run a specific profile with ") + tipKey("fuku run backend"),
	tipDesc("Run without TUI using ") + tipKey("fuku --no-ui"),
	tipDesc("Use ") + tipKey("j/k") + tipDesc(" or arrows to navigate"),
	tipDesc("Press ") + tipKey("s") + tipDesc(" to stop or start a service"),
	tipDesc("Press ") + tipKey("r") + tipDesc(" to restart the selected service"),
	tipDesc("Press ") + tipKey("t") + tipDesc(" to hide these tips"),
}
