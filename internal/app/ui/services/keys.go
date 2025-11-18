package services

import (
	"github.com/charmbracelet/bubbles/key"

	"fuku/internal/app/ui/logs"
)

// KeyMap defines the key bindings for the services TUI
type KeyMap struct {
	Up                  key.Binding
	Down                key.Binding
	Restart             key.Binding
	Stop                key.Binding
	ToggleLogStream     key.Binding
	ToggleAllLogStreams key.Binding
	ToggleLogs          key.Binding
	Autoscroll          key.Binding
	Quit                key.Binding
	ForceQuit           key.Binding
}

// ServicesKeyMap defines the key bindings help for services view
type ServicesKeyMap struct {
	Up                  key.Binding
	Down                key.Binding
	Stop                key.Binding
	Restart             key.Binding
	ToggleLogStream     key.Binding
	ToggleAllLogStreams key.Binding
	ToggleLogs          key.Binding
	Quit                key.Binding
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
		ToggleLogStream: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		ToggleAllLogStreams: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("ctrl+a", "toggle all"),
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

// ServicesHelpKeyMap returns key bindings for services view help
func ServicesHelpKeyMap(k KeyMap) ServicesKeyMap {
	logsBinding := k.ToggleLogs
	logsBinding.SetHelp("l", "logs view")

	return ServicesKeyMap{
		Up:                  k.Up,
		Down:                k.Down,
		Stop:                k.Stop,
		Restart:             k.Restart,
		ToggleLogStream:     k.ToggleLogStream,
		ToggleAllLogStreams: k.ToggleAllLogStreams,
		ToggleLogs:          logsBinding,
		Quit:                k.Quit,
	}
}

// LogsHelpKeyMap returns key bindings for logs view help
func LogsHelpKeyMap(k KeyMap) logs.KeyMap {
	logsBinding := k.ToggleLogs
	logsBinding.SetHelp("l", "services view")

	scrollUp := k.Up
	scrollUp.SetHelp("↑/k", "scroll up")

	scrollDown := k.Down
	scrollDown.SetHelp("↓/j", "scroll down")

	return logs.KeyMap{
		Up:         scrollUp,
		Down:       scrollDown,
		Autoscroll: k.Autoscroll,
		ToggleLogs: logsBinding,
		Quit:       k.Quit,
	}
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Stop, k.Restart, k.ToggleLogStream, k.ToggleAllLogStreams, k.Up, k.Down, k.ToggleLogs, k.Autoscroll, k.Quit}
}

// FullHelp returns keybindings for the expanded help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Stop, k.Restart, k.ToggleLogStream, k.ToggleAllLogStreams, k.Up, k.Down, k.ToggleLogs, k.Autoscroll, k.Quit, k.ForceQuit},
	}
}

// ShortHelp returns keybindings for services view mini help
func (k ServicesKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Stop, k.Restart, k.ToggleLogStream, k.ToggleAllLogStreams, k.ToggleLogs, k.Quit}
}

// FullHelp returns keybindings for services view expanded help
func (k ServicesKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Stop, k.Restart, k.ToggleLogStream, k.ToggleAllLogStreams, k.ToggleLogs, k.Quit},
	}
}
