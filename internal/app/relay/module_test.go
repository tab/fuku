package relay

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"io"

	"fuku/internal/app/instance"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

type testLifecycle struct {
	hooks []fx.Hook
}

func (l *testLifecycle) Append(hook fx.Hook) {
	l.hooks = append(l.hooks, hook)
}

func Test_startBridge(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBroadcaster := NewMockBroadcaster(ctrl)
	formatter := bus.NewFormatter(logger.NewEventLogger())
	b := bus.NoOp()

	bridge := NewBridge(b, mockBroadcaster, formatter)

	lc := &testLifecycle{}
	ctx := context.Background()
	startBridge(lc, ctx, bridge)

	require.Len(t, lc.hooks, 1)

	err := lc.hooks[0].OnStart(context.Background())
	require.NoError(t, err)
}

func Test_startServer(t *testing.T) {
	cfg := config.DefaultConfig()
	b := bus.NoOp()
	log := logger.NewLoggerWithOutput(cfg, io.Discard)

	identity := instance.Identity{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     "/Users/dev/projects/shop",
		Fingerprint: instance.Fingerprint("/Users/dev/projects/shop"),
	}

	server := NewServer(cfg, b, identity, log)

	lc := &testLifecycle{}
	ctx := context.Background()
	startServer(lc, ctx, server)

	require.Len(t, lc.hooks, 1)
	require.NotNil(t, lc.hooks[0].OnStart)
	require.NotNil(t, lc.hooks[0].OnStop)

	err := lc.hooks[0].OnStop(context.Background())
	require.NoError(t, err)
}
