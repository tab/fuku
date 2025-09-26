package cli

import "go.uber.org/fx"

// Module provides the fx dependency injection options for the cli package
var Module = fx.Options(
	fx.Provide(NewCLI),
)
