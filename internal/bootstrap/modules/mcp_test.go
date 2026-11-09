package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/logsocket"
	"fuku/internal/adapters/mcp"
	"fuku/internal/adapters/rest"
	"fuku/internal/model"
)

func Test_MCP_Dialer(t *testing.T) {
	var dial mcp.Dialer

	identity := model.Instance{Project: t.TempDir(), Fingerprint: "this-project"}

	app := fx.New(MCP(&cli.Options{Type: cli.CommandMCP}, model.Telemetry{}), fx.Supply(SentryDSN("")), fx.Populate(&dial))

	require.NoError(t, app.Err())
	assert.IsType(t, &logsocket.Client{}, dial(identity))
}

func Test_MCP_APIDialer(t *testing.T) {
	var dial mcp.APIDialer

	app := fx.New(MCP(&cli.Options{Type: cli.CommandMCP}, model.Telemetry{}), fx.Supply(SentryDSN("")), fx.Populate(&dial))

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
			api, err := dial(tt.address, "")

			if tt.expectedError != nil {
				require.ErrorIs(t, err, tt.expectedError)
				assert.Nil(t, api)
				assert.Equal(t, mcp.API(nil), api)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, api)
		})
	}
}
