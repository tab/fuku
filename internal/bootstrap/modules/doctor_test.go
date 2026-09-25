package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/terminal"
	"fuku/internal/adapters/tui"
)

func Test_newDoctorRenderer(t *testing.T) {
	theme := func() terminal.Theme { return terminal.NewTheme(terminal.AppearanceLight) }

	tests := []struct {
		name     string
		format   cli.Format
		expected cli.Renderer
	}{
		{
			name:     "text selects the styled report",
			format:   cli.FormatText,
			expected: &tui.Report{},
		},
		{
			name:     "summary selects the styled summary",
			format:   cli.FormatSummary,
			expected: &tui.Summary{},
		},
		{
			name:     "json selects the JSON renderer",
			format:   cli.FormatJSON,
			expected: &cli.JSON{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.IsType(t, tt.expected, newDoctorRenderer(tt.format, theme))
		})
	}
}
