package modules

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/logsocket"
	"fuku/internal/adapters/mcp"
	"fuku/internal/adapters/rest"
	"fuku/internal/adapters/terminal"
	"fuku/internal/adapters/tui"
	"fuku/internal/model"
)

func Test_MCP_Instance(t *testing.T) {
	var instance func(model.Instance) mcp.Instance

	identity := model.Instance{Project: t.TempDir(), Fingerprint: "this-project"}

	app := fx.New(MCP(&cli.Options{Type: cli.CommandMCP}, model.Telemetry{}), fx.Supply(SentryDSN("")), fx.Populate(&instance))

	require.NoError(t, app.Err())
	assert.IsType(t, &logsocket.Client{}, instance(identity))
}

func Test_MCP_Client(t *testing.T) {
	var client func(address, token string) (mcp.Client, error)

	app := fx.New(MCP(&cli.Options{Type: cli.CommandMCP}, model.Telemetry{}), fx.Supply(SentryDSN("")), fx.Populate(&client))

	require.NoError(t, app.Err())

	tests := []struct {
		name          string
		address       string
		expectedError error
	}{
		{name: "a loopback address", address: "127.0.0.1:3858"},
		{name: "an address off the loopback", address: "192.168.1.2:3858", expectedError: rest.ErrClientNotLoopback},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client(tt.address, "")

			if tt.expectedError != nil {
				require.ErrorIs(t, err, tt.expectedError)
				assert.Nil(t, result)
				assert.Equal(t, mcp.Client(nil), result)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
}

func Test_pickConsole(t *testing.T) {
	plain := mcp.NewPlain(io.Discard)
	live := tui.NewMCPConsole(terminal.NewTheme(terminal.AppearanceDark), io.Discard)

	tests := []struct {
		name       string
		noUI       bool
		isTerminal bool
		expected   mcp.Console
	}{
		{
			name:       "--no-ui on a terminal prints plain lines",
			noUI:       true,
			isTerminal: true,
			expected:   plain,
		},
		{
			name:     "a stderr that is not a terminal prints plain lines",
			expected: plain,
		},
		{
			name:       "a terminal stderr gets the live console",
			isTerminal: true,
			expected:   live,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Same(t, tt.expected, pickConsole(tt.noUI, tt.isTerminal, plain, live))
		})
	}
}
