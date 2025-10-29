package procstats

import "go.uber.org/fx"

// Module provides the fx dependency injection options for the procstats package
var Module = fx.Options(
	fx.Provide(NewProvider),
)
