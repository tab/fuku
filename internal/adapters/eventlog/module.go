package eventlog

import "go.uber.org/fx"

// Module provides the event log for dependency injection
var Module = fx.Options(
	fx.Provide(NewFormatter),
	fx.Provide(NewRecorder),
)
