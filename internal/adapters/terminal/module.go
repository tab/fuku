package terminal

import "go.uber.org/fx"

// Module provides the log line formatter for dependency injection
var Module = fx.Options(
	fx.Provide(NewLog),
)
