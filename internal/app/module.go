package app

import (
	"go.uber.org/fx"

	"fuku/internal/app/cli"
	"fuku/internal/app/monitor"
	"fuku/internal/app/runner"
	"fuku/internal/app/runtime"
	"fuku/internal/app/ui"
	"fuku/internal/app/ui/services"
)

// Module provides the fx dependency injection options for the app package
var Module = fx.Options(
	cli.Module,
	monitor.Module,
	runner.Module,
	runtime.Module,
	ui.Module,
	services.Module,
	fx.Provide(NewApp),
	fx.Invoke(Register),
)
