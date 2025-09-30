package readiness

import "go.uber.org/fx"

// Module provides the fx dependency injection options for the readiness package
var Module = fx.Options(
	fx.Provide(NewFactory),
)
