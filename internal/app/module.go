package app

import (
	"go.uber.org/fx"

	"fuku/internal/app/bus"
	"fuku/internal/app/cli"
	"fuku/internal/app/logs"
	"fuku/internal/app/monitor"
	"fuku/internal/app/runner"
	"fuku/internal/app/ui/wire"
	"fuku/internal/app/watcher"
)

// Module provides the fx dependency injection options for the app package
var Module = fx.Options(
	bus.Module,
	cli.Module,
	logs.Module,
	monitor.Module,
	runner.Module,
	watcher.Module,
	wire.Module,
	fx.Provide(NewApp),
	fx.Invoke(Register),
)
