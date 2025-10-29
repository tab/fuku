package state

import "go.uber.org/fx"

// Module provides the fx dependency injection options for the state package
var Module = fx.Options(
	fx.Provide(NewManager),
)
