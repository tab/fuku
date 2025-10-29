package cli

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type helpModel struct{}

func newHelpModel() helpModel {
	return helpModel{}
}

func (m helpModel) Init() tea.Cmd {
	return nil
}

func (m helpModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m helpModel) View() string {
	usageSection := sectionHeader.Render("Usage:")
	usage := lipgloss.JoinVertical(
		lipgloss.Left,
		bodyMedium.Render("  "+commandName.Render("fuku --run=<PROFILE>")+"            Run fuku with the specified profile"),
		bodyMedium.Render("  "+commandName.Render("fuku help")+"                       Show help"),
		bodyMedium.Render("  "+commandName.Render("fuku version")+"                    Show version"),
	)

	examplesSection := sectionHeader.Render("Examples:")
	examples := lipgloss.JoinVertical(
		lipgloss.Left,
		bodyMedium.Render("  "+exampleCode.Render("fuku --run=default")+"              Run all services"),
		bodyMedium.Render("  "+exampleCode.Render("fuku --run=core")+"                 Run core services"),
		bodyMedium.Render("  "+exampleCode.Render("fuku --run=minimal")+"              Run minimal services"),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		RenderTitle(),
		usageSection,
		usage,
		examplesSection,
		examples,
		RenderHelp(),
	) + "\n"
}
