package updater

import (
	"go.uber.org/fx"
)

// Module provides the version checker for dependency injection
var Module = fx.Options(
	fx.Provide(NewChecker),
)
