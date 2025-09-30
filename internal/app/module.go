package app

import (
	"go.uber.org/fx"

	"fuku/internal/app/cli"
	"fuku/internal/app/logs"
	"fuku/internal/app/procstats"
	"fuku/internal/app/readiness"
	"fuku/internal/app/runner"
	"fuku/internal/app/state"
	"fuku/internal/config/logger"
)

// Module provides the fx dependency injection options for the app package
var Module = fx.Options(
	cli.Module,
	runner.Module,
	logger.Module,
	state.Module,
	logs.Module,
	procstats.Module,
	readiness.Module,
	fx.Provide(NewApp),
	fx.Invoke(Register),
)
