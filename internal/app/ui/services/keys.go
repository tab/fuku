package services

import "charm.land/bubbles/v2/key"

// KeyMap defines the key bindings for the services view
type KeyMap struct {
	Up            key.Binding
	Down          key.Binding
	Stop          key.Binding
	Restart       key.Binding
	RestartFailed key.Binding
	ToggleTips    key.Binding
	Filter        key.Binding
	ClearFilter   key.Binding
	Quit          key.Binding
	ForceQuit     key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Stop: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "stop/start"),
		),
		Restart: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "restart"),
		),
		RestartFailed: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "restart failed"),
		),
		ToggleTips: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "tips"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		ClearFilter: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear filter"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "force quit"),
		),
	}
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Stop, k.Restart, k.RestartFailed, k.Filter, k.ClearFilter, k.Quit}
}

// FullHelp returns keybindings for the expanded help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Stop, k.Restart, k.RestartFailed, k.Filter, k.ClearFilter, k.Quit},
	}
}
