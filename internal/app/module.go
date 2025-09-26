package app

import (
	"go.uber.org/fx"

	"fuku/internal/app/cli"
	"fuku/internal/app/runner"
	"fuku/internal/config/logger"
)

// Module provides the fx dependency injection options for the app package
var Module = fx.Options(
	cli.Module,
	runner.Module,
	logger.Module,
	fx.Provide(NewApp),
	fx.Invoke(Register),
)
