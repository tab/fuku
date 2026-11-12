package mcp

import "go.uber.org/fx"

// Module provides the MCP server, the fuku binary runner and the plain console for dependency injection
var Module = fx.Options(
	fx.Provide(
		NewServer,
		NewBinary,
		fx.Annotate(NewPlain, fx.ParamTags(`name:"stderr"`)),
	),
)
