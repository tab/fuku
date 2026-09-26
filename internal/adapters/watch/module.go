package watch

import "go.uber.org/fx"

// Module provides the file watcher for dependency injection
var Module = fx.Options(
	fx.Provide(NewWatcher),
)
