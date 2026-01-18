package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DefaultKeyMap(t *testing.T) {
	km := DefaultKeyMap()

	assert.NotEmpty(t, km.Up.Keys())
	assert.NotEmpty(t, km.Down.Keys())
	assert.NotEmpty(t, km.Quit.Keys())
	assert.NotEmpty(t, km.ForceQuit.Keys())

	assert.Contains(t, km.Up.Keys(), "up")
	assert.Contains(t, km.Up.Keys(), "k")
	assert.Contains(t, km.Down.Keys(), "down")
	assert.Contains(t, km.Down.Keys(), "j")
	assert.Contains(t, km.Quit.Keys(), "q")
	assert.Contains(t, km.ForceQuit.Keys(), "ctrl+c")
}
