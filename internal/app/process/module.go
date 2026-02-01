package process

import "go.uber.org/fx"

// Module provides the process and its dependencies
var Module = fx.Options(
	fx.Provide(
		NewProcess,
	),
)
