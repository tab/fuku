package modules

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/detach"
	"fuku/internal/adapters/terminal"
	"fuku/internal/adapters/tui"
	"fuku/internal/model"
	"fuku/internal/platform/logging"
)

func Test_Detach_StopTimeout(t *testing.T) {
	project := model.Project{
		Logging:     model.Logging{Level: logging.LevelInfo, Format: logging.FormatJSON},
		Concurrency: model.Concurrency{Workers: 5},
		Services:    []model.Service{{Name: "api"}, {Name: "db"}},
	}
	cmd := &cli.Options{Type: cli.CommandRun, Profile: model.ProfileDefault, Detached: true}
	child := &cli.Options{Type: cli.CommandRun, Profile: model.ProfileDefault, NoUI: true, DetachedChild: true}

	parent := fx.New(Detach(cmd, project), fx.Supply(SentryDSN("")))
	run := fx.New(Run(child, project), fx.Supply(SentryDSN("")))

	require.NoError(t, parent.Err())
	require.NoError(t, run.Err())
	assert.Greater(t, parent.StopTimeout(), run.StopTimeout())
}

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
