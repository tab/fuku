package tui

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/adapters/terminal"
	"fuku/internal/contracts"
)

func Test_LogView_banner(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceLight)

	var buf bytes.Buffer

	view := NewLogView(theme, nil, &buf, 80)

	tests := []struct {
		name       string
		status     contracts.LogStatus
		subscribed []string
		expects    []string
	}{
		{
			name: "all services",
			status: contracts.LogStatus{
				Profile:  "default",
				Version:  "1.0.0",
				Services: []string{"api", "web", "worker"},
			},
			subscribed: nil,
			expects:    []string{"logs", "default", "3 running", "all", "v1.0.0", "ctrl+c", "exit"},
		},
		{
			name: "filtered services",
			status: contracts.LogStatus{
				Profile:  "backend",
				Version:  "1.0.0",
				Services: []string{"api", "web", "worker"},
			},
			subscribed: []string{"api", "web"},
			expects:    []string{"backend", "api, web"},
		},
		{
			name: "more than 5 services truncated",
			status: contracts.LogStatus{
				Profile:  "all",
				Version:  "1.0.0",
				Services: []string{"a", "b", "c", "d", "e", "f", "g", "h"},
			},
			subscribed: []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7"},
			expects:    []string{"s1, s2, s3, s4, s5", "and 2 more"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			view.banner(tt.status, tt.subscribed)

			for _, expected := range tt.expects {
				assert.Contains(t, buf.String(), expected)
			}
		})
	}
}

func Test_TerminalWidth_NotATerminal(t *testing.T) {
	stdout := os.Stdout

	file, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.Stdout = stdout

		file.Close()
	})

	os.Stdout = file

	width := TerminalWidth()

	assert.Equal(t, 80, width)
}
