package tui

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"testing/iotest"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/adapters/terminal"
	"fuku/internal/contracts"
)

// queue is a Subscription over a plain channel
type queue chan contracts.Message

func (q queue) Messages() <-chan contracts.Message {
	return q
}

func Test_Program_Run(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	theme := func() terminal.Theme { return terminal.NewTheme(terminal.AppearanceDark) }

	errInput := errors.New("input failed")
	dropAll := func(tea.Model, tea.Msg) tea.Msg { return nil }

	ended, cancel := context.WithCancel(t.Context())
	cancel()

	tests := []struct {
		name          string
		before        func() *Program
		ctx           context.Context
		expectedExit  int
		expectedError error
	}{
		{
			name: "the context ends",
			before: func() *Program {
				program := NewProgram(ProgramParams{
					Options: Options{Profile: "default"},
					Bridge:  NewBridge(nil),
					Theme:   theme,
					Logger:  log,
				})
				program.options = []tea.ProgramOption{tea.WithInput(nil), tea.WithoutRenderer()}

				return program
			},
			ctx:          ended,
			expectedExit: 0,
		},
		{
			name: "a failed input returns the error",
			before: func() *Program {
				program := NewProgram(ProgramParams{
					Options: Options{Profile: "default"},
					Bridge:  NewBridge(nil),
					Theme:   theme,
					Logger:  log,
				})
				program.options = []tea.ProgramOption{tea.WithInput(iotest.ErrReader(errInput)), tea.WithoutRenderer(), tea.WithFilter(dropAll)}

				return program
			},
			ctx:           t.Context(),
			expectedExit:  1,
			expectedError: errInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := tt.before()

			exitCode, err := program.Run(tt.ctx)

			require.ErrorIs(t, err, tt.expectedError)
			assert.Equal(t, tt.expectedExit, exitCode)
			assert.NotNil(t, program.params.Bridge.view)
		})
	}
}
