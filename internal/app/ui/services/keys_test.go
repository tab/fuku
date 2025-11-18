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
	assert.NotEmpty(t, km.ToggleLogStream.Keys())
	assert.NotEmpty(t, km.ToggleAllLogStreams.Keys())
	assert.NotEmpty(t, km.ToggleLogs.Keys())
	assert.NotEmpty(t, km.Autoscroll.Keys())
	assert.NotEmpty(t, km.Quit.Keys())
	assert.NotEmpty(t, km.ForceQuit.Keys())

	assert.Contains(t, km.Up.Keys(), "up")
	assert.Contains(t, km.Up.Keys(), "k")
	assert.Contains(t, km.Down.Keys(), "down")
	assert.Contains(t, km.Down.Keys(), "j")
	assert.Contains(t, km.Stop.Keys(), "s")
	assert.Contains(t, km.Restart.Keys(), "r")
	assert.Contains(t, km.ToggleLogStream.Keys(), " ")
	assert.Contains(t, km.ToggleAllLogStreams.Keys(), "ctrl+a")
	assert.Contains(t, km.ToggleLogs.Keys(), "tab")
	assert.Contains(t, km.Autoscroll.Keys(), "a")
	assert.Contains(t, km.Quit.Keys(), "q")
	assert.Contains(t, km.ForceQuit.Keys(), "ctrl+c")
}

func Test_KeyMap_ShortHelp(t *testing.T) {
	km := DefaultKeyMap()
	bindings := km.ShortHelp()

	assert.Len(t, bindings, 9)
	assert.Equal(t, km.Stop, bindings[0])
	assert.Equal(t, km.Restart, bindings[1])
	assert.Equal(t, km.ToggleLogStream, bindings[2])
	assert.Equal(t, km.ToggleAllLogStreams, bindings[3])
	assert.Equal(t, km.Up, bindings[4])
	assert.Equal(t, km.Down, bindings[5])
	assert.Equal(t, km.ToggleLogs, bindings[6])
	assert.Equal(t, km.Autoscroll, bindings[7])
	assert.Equal(t, km.Quit, bindings[8])
}

func Test_KeyMap_FullHelp(t *testing.T) {
	km := DefaultKeyMap()
	bindings := km.FullHelp()

	assert.Len(t, bindings, 1)
	assert.Len(t, bindings[0], 10)
	assert.Equal(t, km.Stop, bindings[0][0])
	assert.Equal(t, km.Restart, bindings[0][1])
	assert.Equal(t, km.ToggleLogStream, bindings[0][2])
	assert.Equal(t, km.ToggleAllLogStreams, bindings[0][3])
	assert.Equal(t, km.Up, bindings[0][4])
	assert.Equal(t, km.Down, bindings[0][5])
	assert.Equal(t, km.ToggleLogs, bindings[0][6])
	assert.Equal(t, km.Autoscroll, bindings[0][7])
	assert.Equal(t, km.Quit, bindings[0][8])
	assert.Equal(t, km.ForceQuit, bindings[0][9])
}
