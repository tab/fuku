package runner

import "go.uber.org/fx"

// Module provides the fx dependency injection options for the runner package
var Module = fx.Options(
	fx.Provide(NewRunner),
)
