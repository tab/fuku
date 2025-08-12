package app

import (
	"go.uber.org/fx"

	"fuku/internal/app/cli"
	"fuku/internal/config/logger"
)

var Module = fx.Options(
	cli.Module,
	logger.Module,
	fx.Provide(NewApp),
	fx.Invoke(Register),
)
