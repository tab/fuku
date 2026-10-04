package detach

import "go.uber.org/fx"

// Module provides the detached run for dependency injection
var Module = fx.Options(
	fx.Provide(NewStderr),
	fx.Provide(NewProgress),
	fx.Provide(NewLauncher),
	fx.Provide(fx.Annotate(NewStopper, fx.ParamTags(``, ``, `name:"stdout"`))),
	fx.Provide(fx.Annotate(NewPlain, fx.ParamTags(`name:"stdout"`))),
	fx.Provide(fx.Annotate(NewCommand, fx.ParamTags(``, ``, `name:"stdout"`, `name:"stderr"`))),
)
