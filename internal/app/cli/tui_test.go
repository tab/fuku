package cli

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewTUI(t *testing.T) {
	cfg := &config.Config{}
	log := logger.NewLogger(&config.Config{})
	tuiInstance := NewTUI(cfg, log)
	assert.NotNil(t, tuiInstance)
	assert.Implements(t, (*TUI)(nil), tuiInstance)
}

func Test_newRootModel(t *testing.T) {
	tests := []struct {
		name         string
		viewType     viewType
		expectActive bool
	}{
		{
			name:         "Help view",
			viewType:     helpView,
			expectActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newRootModel(tt.viewType)

			assert.Equal(t, tt.viewType, m.viewType)
			if tt.expectActive {
				assert.NotNil(t, m.activeView)
			} else {
				assert.Nil(t, m.activeView)
			}
		})
	}
}

func Test_rootModel_Init(t *testing.T) {
	tests := []struct {
		name     string
		viewType viewType
	}{
		{
			name:     "Help view",
			viewType: helpView,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newRootModel(tt.viewType)
			cmd := m.Init()
			assert.Nil(t, cmd)
		})
	}
}

func Test_rootModel_Update(t *testing.T) {
	m := newRootModel(helpView)

	tests := []struct {
		name        string
		msg         tea.Msg
		expectQuit  bool
		expectModel tea.Model
	}{
		{
			name:        "Quit on ctrl+c",
			msg:         tea.KeyMsg{Type: tea.KeyCtrlC},
			expectQuit:  true,
			expectModel: m,
		},
		{
			name:        "Delegate q to active view",
			msg:         tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}},
			expectQuit:  true,
			expectModel: m,
		},
		{
			name:        "Delegate esc to active view",
			msg:         tea.KeyMsg{Type: tea.KeyEsc},
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
			name:        "Delegate non-key message to active view",
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

func Test_rootModel_View(t *testing.T) {
	tests := []struct {
		name           string
		viewType       viewType
		expectContains []string
	}{
		{
			name:     "Help view",
			viewType: helpView,
			expectContains: []string{
				"Fuku",
				"Usage:",
				"Examples:",
				"Press q or esc to exit",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newRootModel(tt.viewType)
			view := m.View()

			assert.NotEmpty(t, view)
			for _, expected := range tt.expectContains {
				assert.Contains(t, view, expected)
			}
		})
	}
}

func Test_rootModel_View_NoActiveView(t *testing.T) {
	m := rootModel{
		activeView: nil,
		viewType:   helpView,
	}

	view := m.View()
	assert.Empty(t, view)
}
