package modules

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/terminal"
	"fuku/internal/adapters/tui"
	"fuku/internal/platform/logging"
)

func Test_newLogsView(t *testing.T) {
	theme := func() terminal.Theme { return terminal.NewTheme(terminal.AppearanceLight) }
	log := terminal.NewLog(terminal.Options{Format: logging.FormatConsole}, theme)

	tests := []struct {
		name     string
		cmd      *cli.Options
		expected any
	}{
		{
			name:     "without a UI the lines are bare",
			cmd:      &cli.Options{Type: cli.CommandLogs, NoUI: true},
			expected: &cli.LogView{},
		},
		{
			name:     "with the UI the banner and styled lines render inline",
			cmd:      &cli.Options{Type: cli.CommandLogs},
			expected: &tui.LogView{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := newLogsView(tt.cmd, theme, log, io.Discard)

			assert.IsType(t, tt.expected, view)
		})
	}
}
