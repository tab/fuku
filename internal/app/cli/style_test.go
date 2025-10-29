package cli

import (
	"fuku/internal/config"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_RenderTitle(t *testing.T) {
	result := RenderTitle()

	assert.NotEmpty(t, result)
	assert.Contains(t, result, config.AppName)
	assert.Contains(t, result, config.Version)
	assert.Contains(t, result, config.AppDescription)
}

func Test_RenderHelp(t *testing.T) {
	result := RenderHelp()

	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Press q or esc to exit")
}
