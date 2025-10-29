package cli

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

type viewType int

const (
	helpView viewType = iota
)

type TUI interface {
	Help() error
	Run(ctx context.Context, profile string) error
}

type tui struct {
	cfg *config.Config
	log logger.Logger
}

func NewTUI(cfg *config.Config, log logger.Logger) TUI {
	return &tui{
		cfg: cfg,
		log: log,
	}
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

func (t *tui) Run(ctx context.Context, profile string) error {
	silentLog := logger.NewSilentLogger(t.cfg)

	p := tea.NewProgram(
		newRunModel(ctx, t.cfg, silentLog, profile),
		tea.WithAltScreen(),
	)

	_, err := p.Run()

	return err
}
