package bus

import "go.uber.org/fx"

// Module provides the bus transport for dependency injection
var Module = fx.Options(
	fx.Provide(NewBus),
)
