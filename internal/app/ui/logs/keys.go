package logs

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the key bindings help for logs view
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Autoscroll key.Binding
	ToggleLogs key.Binding
	Quit       key.Binding
}

// ShortHelp returns keybindings for logs view mini help
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Autoscroll, k.ToggleLogs, k.Quit}
}

// FullHelp returns keybindings for logs view expanded help
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Autoscroll, k.ToggleLogs, k.Quit},
	}
}
