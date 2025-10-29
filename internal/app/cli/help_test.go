package cli

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func Test_newHelpModel(t *testing.T) {
	m := newHelpModel()
	assert.NotNil(t, m)
}

func Test_helpModel_Init(t *testing.T) {
	m := newHelpModel()
	cmd := m.Init()
	assert.Nil(t, cmd)
}

func Test_helpModel_Update(t *testing.T) {
	m := newHelpModel()

	tests := []struct {
		name        string
		msg         tea.Msg
		expectQuit  bool
		expectModel tea.Model
	}{
		{
			name:        "Quit on q",
			msg:         tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}},
			expectQuit:  true,
			expectModel: m,
		},
		{
			name:        "Quit on esc",
			msg:         tea.KeyMsg{Type: tea.KeyEsc},
			expectQuit:  true,
			expectModel: m,
		},
		{
			name:        "Quit on ctrl+c",
			msg:         tea.KeyMsg{Type: tea.KeyCtrlC},
			expectQuit:  true,
			expectModel: m,
		},
		{
			name:        "No quit on other key",
			msg:         tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}},
			expectQuit:  false,
			expectModel: m,
		},
		{
			name:        "No quit on non-key message",
			msg:         tea.WindowSizeMsg{Width: 80, Height: 24},
			expectQuit:  false,
			expectModel: m,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, cmd := m.Update(tt.msg)

			assert.Equal(t, tt.expectModel, model)

			if tt.expectQuit {
				assert.NotNil(t, cmd)
			} else {
				assert.Nil(t, cmd)
			}
		})
	}
}

func Test_helpModel_View(t *testing.T) {
	m := newHelpModel()
	view := m.View()

	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Fuku")
	assert.Contains(t, view, "Usage:")
	assert.Contains(t, view, "Examples:")
	assert.Contains(t, view, "fuku --run=")
	assert.Contains(t, view, "fuku help")
	assert.Contains(t, view, "fuku version")
	assert.Contains(t, view, "Press q or esc to exit")
}
