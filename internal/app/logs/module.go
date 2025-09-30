package logs

import "go.uber.org/fx"

// Module provides the fx dependency injection options for the logs package
var Module = fx.Options(
	fx.Provide(NewManager),
)
