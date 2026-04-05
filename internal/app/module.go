package app

import (
	"go.uber.org/fx"

	"fuku/internal/app/bus"
	"fuku/internal/app/cli"
	"fuku/internal/app/logs"
	"fuku/internal/app/metrics"
	"fuku/internal/app/monitor"
	"fuku/internal/app/relay"
	"fuku/internal/app/runner"
	"fuku/internal/app/sampler"
	"fuku/internal/app/tracer"
	"fuku/internal/app/ui/wire"
	"fuku/internal/app/watcher"
)

// Module provides the fx dependency injection options for the app package
var Module = fx.Options(
	bus.Module,
	cli.Module,
	logs.Module,
	metrics.Module,
	monitor.Module,
	relay.Module,
	runner.Module,
	sampler.Module,
	tracer.Module,
	watcher.Module,
	wire.Module,
	fx.Provide(provideContext),
	fx.Provide(NewApp),
	fx.Invoke(Register),
)
