package logs

import (
	"context"

	"go.uber.org/fx"

	"fuku/internal/app/runtime"
	"fuku/internal/app/ui"
)

// Module provides the logs UI components
var Module = fx.Options(
	fx.Provide(
		NewLogView,
	),
	fx.Invoke(startLogSubscriber),
)

// NewLogView creates a new LogView implementation
func NewLogView() ui.LogView {
	model := NewModel()

	return &model
}

func startLogSubscriber(lc fx.Lifecycle, eventBus runtime.EventBus, logView ui.LogView) {
	ctx, cancel := context.WithCancel(context.Background())

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			subscriber := NewSubscriber(eventBus, logView)
			subscriber.Start(ctx)

			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()

			return nil
		},
	})
}
