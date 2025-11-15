package services

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the key bindings for the services TUI
type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Restart      key.Binding
	Stop         key.Binding
	ToggleLogSub key.Binding
	ToggleLogs   key.Binding
	Autoscroll   key.Binding
	Quit         key.Binding
	ForceQuit    key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Stop: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "stop/start"),
		),
		Restart: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "restart"),
		),
		ToggleLogSub: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle log"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		ToggleLogs: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "logs view"),
		),
		Autoscroll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "autoscroll"),
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
	return []key.Binding{k.Stop, k.Restart, k.ToggleLogSub, k.Up, k.Down, k.ToggleLogs, k.Autoscroll, k.Quit}
}

// FullHelp returns keybindings for the expanded help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Stop, k.Restart, k.ToggleLogSub, k.Up, k.Down, k.ToggleLogs, k.Autoscroll, k.Quit, k.ForceQuit},
	}
}
