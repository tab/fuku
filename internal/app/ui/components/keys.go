package components

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines shared key bindings used across all views
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	ToggleLogs key.Binding
	Quit       key.Binding
	ForceQuit  key.Binding
}

// DefaultKeyMap returns the default shared key bindings
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
		ToggleLogs: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle view"),
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
