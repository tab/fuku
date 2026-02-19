package session

import "go.uber.org/fx"

// Module provides the fx dependency injection options for the session package
var Module = fx.Options(
	fx.Provide(NewSession),
	fx.Provide(NewListener),
)
