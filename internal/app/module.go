package app

import (
	"go.uber.org/fx"

	"fuku/internal/app/cli"
	"fuku/internal/app/runner"
	"fuku/internal/config/logger"
)

var Module = fx.Options(
	cli.Module,
	runner.Module,
	logger.Module,
	fx.Provide(NewApp),
	fx.Invoke(Register),
)
