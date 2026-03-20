package relay

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
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
	startBridge(lc, bridge)

	require.Len(t, lc.hooks, 1)

	err := lc.hooks[0].OnStart(context.Background())
	require.NoError(t, err)

	err = lc.hooks[0].OnStop(context.Background())
	require.NoError(t, err)
}
