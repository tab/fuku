package mcp

import "go.uber.org/fx"

// Module provides the MCP server and the fuku binary runner for dependency injection
var Module = fx.Options(
	fx.Provide(
		fx.Annotate(NewServer, fx.ParamTags(``, ``, ``, ``, ``, `name:"stderr"`)),
		NewBinary,
	),
)
