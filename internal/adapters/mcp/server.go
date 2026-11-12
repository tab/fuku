package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"fuku/internal/app/logs"
	"fuku/internal/contracts"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

// instructions tell the agent how to drive fuku; the first 512 characters stand alone, Claude Code reads up to 4096
const instructions = `Control fuku only through these tools: never the fuku CLI, curl, the REST API or a script, not even to probe.
If no tool fits the task, change nothing and tell the user which tool is missing.
Read config with the config tool; grep a config file only to edit one key; never read the API token.
One fuku per project: call status before run or stop, not service or logs; run only when nothing runs, stop only when the user asks.
Call doctor only after a failed run or a config edit.

Tools: status shows each service, or running false and the profiles to run.
config shows the effective config of a service or profile.
logs reads the newest lines and never follows.
run starts a profile in the background.
stop stops fuku.
service starts, stops or restarts one service.
doctor diagnoses a failed run or a config edit.
After a code edit, a watching service restarts itself: wait in status for its new PID, then test.
Restart an edited service that is not watching.
Apply a config change with stop then run only when status reports mode detached. Otherwise tell the developer to restart their fuku.
Leave fuku running when the task is done.`

// Instance reads the status frame and a bounded log read of the running instance of one project
type Instance interface {
	Status() (contracts.LogStatus, error)
	Connect() error
	Subscribe(services []string, replay model.ReplayOptions) error
	Stream(ctx context.Context, handler logs.Handler) error
	Close() error
}

// Loader loads the config of a project directory: the default lookup with its override, or the file path names alone
type Loader func(dir, path string) model.Config

// Executor runs the fuku binary in a project directory and returns its stdout, also when it exits non-zero
type Executor interface {
	Execute(ctx context.Context, dir string, args ...string) ([]byte, error)
}

// Server serves the fuku tools over stdio until stdin closes or the run ends
type Server struct {
	instance func(identity model.Instance) Instance
	client   func(address, token string) (Client, error)
	load     Loader
	execute  Executor
	server   *sdk.Server
	console  Console
}

// NewServer creates the MCP server with its seven tools, showing its banner and one line per tool call on the console
func NewServer(instance func(identity model.Instance) Instance, client func(address, token string) (Client, error), load Loader, execute Executor, console Console) (*Server, error) {
	s := &Server{
		instance: instance,
		client:   client,
		load:     load,
		execute:  execute,
		server: sdk.NewServer(
			&sdk.Implementation{Name: buildinfo.AppName, Version: buildinfo.Version},
			&sdk.ServerOptions{Instructions: instructions, Capabilities: &sdk.ServerCapabilities{}},
		),
		console: console,
	}

	if err := s.register(); err != nil {
		return nil, fmt.Errorf("failed to register the MCP tools: %w", err)
	}

	s.server.AddReceivingMiddleware(s.redactCalls, s.logCalls)

	return s, nil
}

// Run enters the project root, so the env files load from it, opens the console with the banner, then serves stdio
func (s *Server) Run(ctx context.Context) (int, error) {
	if err := os.Chdir(root()); err != nil {
		return 1, fmt.Errorf("failed to enter the project root: %w", err)
	}

	s.console.Open()
	defer s.console.Close()

	s.banner()

	err := s.server.Run(ctx, &sdk.StdioTransport{})
	if err != nil && !errors.Is(err, context.Canceled) {
		return 1, fmt.Errorf("failed to serve MCP: %w", err)
	}

	return 0, nil
}
