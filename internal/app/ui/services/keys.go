package services

import (
	"github.com/charmbracelet/bubbles/key"

	"fuku/internal/app/ui/components"
)

// KeyMap defines the key bindings for the services view
type KeyMap struct {
	components.KeyMap
	Stop                key.Binding
	Restart             key.Binding
	ToggleLogStream     key.Binding
	ToggleAllLogStreams key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	base := components.DefaultKeyMap()

	base.Up.SetHelp("↑/k", "up")
	base.Down.SetHelp("↓/j", "down")
	base.ToggleLogs.SetHelp("tab", "logs view")

	return KeyMap{
		KeyMap: base,
		Stop: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "stop/start"),
		),
		Restart: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "restart"),
		),
		ToggleLogStream: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		ToggleAllLogStreams: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("ctrl+a", "toggle all"),
		),
	}
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Stop, k.Restart, k.ToggleLogStream, k.ToggleAllLogStreams, k.ToggleLogs, k.Quit}
}

// FullHelp returns keybindings for the expanded help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Stop, k.Restart, k.ToggleLogStream, k.ToggleAllLogStreams, k.ToggleLogs, k.Quit},
	}
}
