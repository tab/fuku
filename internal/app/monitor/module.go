package monitor

import "go.uber.org/fx"

// Module provides the monitor and its dependencies
var Module = fx.Options(
	fx.Provide(
		NewMonitor,
		fx.Annotate(
			NewMonitor,
			fx.ResultTags(`name:"sampler"`),
		),
	),
)
