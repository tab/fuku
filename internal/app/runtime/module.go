package runtime

import (
	"go.uber.org/fx"

	"fuku/internal/config/logger"
)

// Module provides runtime dependencies for dependency injection
var Module = fx.Module("runtime",
	fx.Provide(
		func(log logger.Logger) EventBus {
			return NewEventBusWithLogger(100, log.WithComponent("EVENT"))
		},
		func() CommandBus { return NewCommandBus(10) },
	),
)
