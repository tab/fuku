package resources

import "go.uber.org/fx"

// Module provides the process monitors and the resource sampler for dependency injection
var Module = fx.Options(
	fx.Provide(
		NewProcessMonitor,
		fx.Annotate(
			NewProcessMonitor,
			fx.ResultTags(`name:"sampler"`),
		),
		NewSampler,
	),
)
