package preflight

import "go.uber.org/fx"

// Module provides the preflight and its dependencies
var Module = fx.Options(
	fx.Provide(NewPreflight),
)
