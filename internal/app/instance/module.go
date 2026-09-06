package instance

import "go.uber.org/fx"

// Module provides the instance identity and its single-instance guard for dependency injection
var Module = fx.Options(
	fx.Provide(
		NewInstance,
		fx.Annotate(
			NewGuard,
			fx.ParamTags(``, ``, `name:"stderr"`),
		),
	),
)
