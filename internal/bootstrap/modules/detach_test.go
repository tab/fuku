package modules

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/detach"
	"fuku/internal/adapters/terminal"
	"fuku/internal/adapters/tui"
)

func Test_newView(t *testing.T) {
	plain := detach.NewPlain(io.Discard)
	live := tui.NewStartup(tui.Options{}, func() terminal.Theme { return terminal.NewTheme(terminal.AppearanceDark) }, io.Discard)

	tests := []struct {
		name string
		cmd  *cli.Options
	}{
		{
			name: "--no-ui prints plain lines",
			cmd:  &cli.Options{NoUI: true},
		},
		{
			name: "a stdout that is not a terminal prints plain lines",
			cmd:  &cli.Options{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Same(t, plain, newView(tt.cmd, plain, live))
		})
	}
}
