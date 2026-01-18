package app

import (
	"go.uber.org/fx"

	"fuku/internal/app/cli"
	"fuku/internal/app/logs"
	"fuku/internal/app/monitor"
	"fuku/internal/app/runner"
	"fuku/internal/app/runtime"
	"fuku/internal/app/ui/wire"
)

// Module provides the fx dependency injection options for the app package
var Module = fx.Options(
	cli.Module,
	logs.Module,
	monitor.Module,
	runner.Module,
	runtime.Module,
	wire.Module,
	fx.Provide(NewApp),
	fx.Invoke(Register),
)
