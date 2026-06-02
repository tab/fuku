package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DefaultKeyMap(t *testing.T) {
	km := DefaultKeyMap()

	assert.NotEmpty(t, km.Up.Keys())
	assert.NotEmpty(t, km.Down.Keys())
	assert.NotEmpty(t, km.Stop.Keys())
	assert.NotEmpty(t, km.Restart.Keys())
	assert.NotEmpty(t, km.RestartFailed.Keys())
	assert.NotEmpty(t, km.ToggleTips.Keys())
	assert.NotEmpty(t, km.OpenAside.Keys())
	assert.NotEmpty(t, km.AsideClose.Keys())
	assert.NotEmpty(t, km.AsideTabNext.Keys())
	assert.NotEmpty(t, km.AsideTabPrev.Keys())
	assert.NotEmpty(t, km.Quit.Keys())
	assert.NotEmpty(t, km.ForceQuit.Keys())

	assert.Contains(t, km.Up.Keys(), "up")
	assert.Contains(t, km.Up.Keys(), "k")
	assert.Contains(t, km.Down.Keys(), "down")
	assert.Contains(t, km.Down.Keys(), "j")
	assert.Contains(t, km.Stop.Keys(), "s")
	assert.Contains(t, km.Restart.Keys(), "r")
	assert.Contains(t, km.RestartFailed.Keys(), "ctrl+r")
	assert.Contains(t, km.ToggleTips.Keys(), "t")
	assert.Contains(t, km.Filter.Keys(), "/")
	assert.Contains(t, km.ClearFilter.Keys(), "esc")
	assert.Contains(t, km.OpenAside.Keys(), "enter")
	assert.Contains(t, km.AsideClose.Keys(), "esc")
	assert.Contains(t, km.AsideTabNext.Keys(), "tab")
	assert.Contains(t, km.AsideTabPrev.Keys(), "shift+tab")
	assert.Contains(t, km.Quit.Keys(), "q")
	assert.Contains(t, km.ForceQuit.Keys(), "ctrl+c")

	assert.Equal(t, "enter", km.OpenAside.Help().Key)
	assert.Equal(t, "info", km.OpenAside.Help().Desc)
	assert.Equal(t, "esc", km.AsideClose.Help().Key)
	assert.Equal(t, "close info", km.AsideClose.Help().Desc)
}

func Test_KeyMap_ShortHelp(t *testing.T) {
	km := DefaultKeyMap()
	bindings := km.ShortHelp()

	assert.Len(t, bindings, 9)
	assert.Equal(t, km.Up, bindings[0])
	assert.Equal(t, km.Down, bindings[1])
	assert.Equal(t, km.Stop, bindings[2])
	assert.Equal(t, km.Restart, bindings[3])
	assert.Equal(t, km.RestartFailed, bindings[4])
	assert.Equal(t, km.OpenAside, bindings[5])
	assert.Equal(t, km.Filter, bindings[6])
	assert.Equal(t, km.ClearFilter, bindings[7])
	assert.Equal(t, km.Quit, bindings[8])
}

func Test_KeyMap_FullHelp(t *testing.T) {
	km := DefaultKeyMap()
	bindings := km.FullHelp()

	assert.Len(t, bindings, 1)
	assert.Len(t, bindings[0], 9)
	assert.Equal(t, km.Up, bindings[0][0])
	assert.Equal(t, km.Down, bindings[0][1])
	assert.Equal(t, km.Stop, bindings[0][2])
	assert.Equal(t, km.Restart, bindings[0][3])
	assert.Equal(t, km.RestartFailed, bindings[0][4])
	assert.Equal(t, km.OpenAside, bindings[0][5])
	assert.Equal(t, km.Filter, bindings[0][6])
	assert.Equal(t, km.ClearFilter, bindings[0][7])
	assert.Equal(t, km.Quit, bindings[0][8])
}

func Test_AsideHelpKeyMap(t *testing.T) {
	km := DefaultKeyMap()
	a := NewAsideHelpKeyMap(km)

	short := a.ShortHelp()
	assert.Len(t, short, 5)
	assert.Equal(t, km.AsideClose, short[0])
	assert.Equal(t, km.AsideTabNext, short[1])
	assert.Equal(t, km.AsideTabPrev, short[2])
	assert.Equal(t, km.FocusToggle, short[3])
	assert.Equal(t, km.Quit, short[4])

	full := a.FullHelp()
	assert.Len(t, full, 1)
	assert.Equal(t, short, full[0])
}
