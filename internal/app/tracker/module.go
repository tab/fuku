package tracker

import "go.uber.org/fx"

// Module provides the fx dependency injection options for the tracker package
var Module = fx.Options(
	fx.Provide(NewResult),
	fx.Provide(NewTracker),
)
