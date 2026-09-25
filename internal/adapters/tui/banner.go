package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"

	"fuku/internal/adapters/terminal"
	"fuku/internal/contracts"
)

// banner writes the connection banner of an accepted stream
func (v *LogView) banner(status contracts.LogStatus, subscribed []string) {
	serviceCount := fmt.Sprintf("%d running", len(status.Services))

	const maxShown = 5

	var showing string

	switch {
	case len(subscribed) == 0:
		showing = "all"
	case len(subscribed) <= maxShown:
		showing = strings.Join(subscribed, ", ")
	default:
		showing = strings.Join(subscribed[:maxShown], ", ") + fmt.Sprintf(" and %d more", len(subscribed)-maxShown)
	}

	theme := v.theme()

	muted := theme.PanelMutedStyle.Render
	bold := terminal.BoldStyle.Render

	field := func(label, value string) string {
		return " " + muted(label) + " " + bold(value)
	}

	contentLines := []string{
		field("profile:", status.Profile),
		field("services:", serviceCount),
		field("showing:", showing),
	}

	lines := terminal.RenderPanelLines(terminal.PanelOptions{
		Title:       "logs",
		Content:     strings.Join(contentLines, "\n"),
		Version:     theme.PanelMutedStyle.Render("v" + status.Version),
		Height:      len(contentLines) + terminal.PanelBorderHeight,
		Width:       TerminalWidth(),
		BorderStyle: terminal.PanelBorderStyle,
	})

	footer := " " + theme.HelpKeyStyle.Render("ctrl+c") + " " + theme.HelpDescStyle.Render("exit")
	lines = append(lines, footer, "")

	for _, line := range lines {
		fmt.Fprintln(v.out, line)
	}
}

// TerminalWidth returns the current terminal width (80 when it is unknown or too narrow)
func TerminalWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w < 40 {
		return 80
	}

	return w
}
