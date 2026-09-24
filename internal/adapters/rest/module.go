package rest

import "go.uber.org/fx"

// Module provides the REST server for dependency injection
var Module = fx.Options(
	fx.Provide(NewServer),
)
