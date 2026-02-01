package readiness

import "go.uber.org/fx"

// Module provides the readiness and its dependencies
var Module = fx.Options(
	fx.Provide(
		NewReadiness,
	),
)
