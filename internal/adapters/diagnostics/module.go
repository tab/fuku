package diagnostics

import "go.uber.org/fx"

// Module provides the doctor's observers for dependency injection
var Module = fx.Options(
	fx.Provide(
		NewEnvironment,
		NewFilesystem,
		NewRuntime,
	),
)
