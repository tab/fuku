package watcher

import "go.uber.org/fx"

// Module provides the watcher and its dependencies
var Module = fx.Options(
	fx.Provide(NewWatcher),
)
