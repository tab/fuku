package logs

import (
	"github.com/charmbracelet/bubbles/key"

	"fuku/internal/app/ui/components"
)

// KeyMap defines the key bindings for logs view
type KeyMap struct {
	components.KeyMap
	Autoscroll key.Binding
	ClearLogs  key.Binding
}

// DefaultKeyMap returns the default key bindings for logs view
func DefaultKeyMap() KeyMap {
	base := components.DefaultKeyMap()

	base.Up.SetHelp("↑/k", "scroll up")
	base.Down.SetHelp("↓/j", "scroll down")
	base.ToggleLogs.SetHelp("tab", "services view")

	return KeyMap{
		KeyMap: base,
		Autoscroll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "autoscroll"),
		),
		ClearLogs: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "clear logs"),
		),
	}
}

// ShortHelp returns keybindings for logs view mini help
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Autoscroll, k.ClearLogs, k.ToggleLogs, k.Quit}
}

// FullHelp returns keybindings for logs view expanded help
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Autoscroll, k.ClearLogs, k.ToggleLogs, k.Quit},
	}
}
