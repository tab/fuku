package runtime

import "go.uber.org/fx"

// Module provides runtime dependencies for dependency injection
var Module = fx.Module("runtime",
	fx.Provide(
		func() EventBus { return NewEventBus(100) },
		func() CommandBus { return NewCommandBus(10) },
	),
)
