package cli

import "go.uber.org/fx"

// Module provides the command announcer and the commands for dependency injection
var Module = fx.Options(
	fx.Provide(
		NewAnnouncer,
		NewRun,
		NewStop,
		NewLogs,
		fx.Annotate(NewDoctor, fx.ParamTags(``, ``, `name:"stdout"`)),
		fx.Annotate(NewHelp, fx.ParamTags(`name:"stdout"`)),
		fx.Annotate(NewVersion, fx.ParamTags(`name:"stdout"`)),
		fx.Annotate(NewInit, fx.ParamTags(``, `name:"stdout"`)),
	),
)
