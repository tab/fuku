package tui

import "go.uber.org/fx"

// Module provides the bridge and the program of the services view
var Module = fx.Options(
	fx.Provide(
		NewBridge,
		NewProgram,
	),
)
