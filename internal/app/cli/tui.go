package cli

import tea "github.com/charmbracelet/bubbletea"

type viewType int

const (
	helpView viewType = iota
)

type TUI interface {
	Help() error
}

type tui struct{}

func NewTUI() TUI {
	return &tui{}
}

type rootModel struct {
	activeView tea.Model
	viewType   viewType
}

func newRootModel(vt viewType) rootModel {
	m := rootModel{viewType: vt}

	switch vt {
	case helpView:
		m.activeView = newHelpModel()
	default:
		m.activeView = nil
	}

	return m
}

func (m rootModel) Init() tea.Cmd {
	if m.activeView != nil {
		return m.activeView.Init()
	}
	return nil
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	if m.activeView != nil {
		var cmd tea.Cmd
		m.activeView, cmd = m.activeView.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m rootModel) View() string {
	if m.activeView != nil {
		return m.activeView.View()
	}
	return ""
}

func (t *tui) Help() error {
	p := tea.NewProgram(
		newRootModel(helpView),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
