package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"fuku/internal/adapters/terminal"
	"fuku/internal/app/logs"
	"fuku/internal/contracts"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

// instructions tell the agent how to drive fuku; the first 512 characters stand alone
const instructions = "fuku runs the local services of this project. " +
	"Control fuku only through these tools. " +
	"Never run `fuku`, `curl` or the REST API from the shell, and never read the API token. " +
	"Never read fuku.yaml or fuku.override.yaml whole; they may hold secrets. Grep one key. " +
	"One fuku runs per project: call status first, run only when nothing runs, stop only when the user asks. " +
	"Call doctor only after a failed run or a config edit, never before a plain run. " +
	"Omit config unless the user names a config file other than fuku.yaml."

// Client reads the status frame and a bounded log read of the running instance of one project
type Client interface {
	Status() (contracts.LogStatus, error)
	Connect() error
	Subscribe(services []string, replay model.ReplayOptions) error
	Stream(ctx context.Context, handler logs.Handler) error
	Close() error
}

// Dialer returns the log client of the project the identity names
type Dialer func(identity model.Instance) Client

// Loader loads the config at path, or the default fuku.yaml and its override of the working directory when empty
type Loader func(path string) model.Config

// Executor runs the fuku binary in a project directory and returns its stdout, also when it exits non-zero
type Executor interface {
	Execute(ctx context.Context, dir string, args ...string) ([]byte, error)
}

// Server serves the fuku tools over stdio until stdin closes or the run ends
type Server struct {
	dial    Dialer
	dialAPI APIDialer
	load    Loader
	execute Executor
	server  *sdk.Server
	console *console
}

// NewServer creates the MCP server with its six tools, writing its banner and one line per tool call to stderr
func NewServer(dial Dialer, dialAPI APIDialer, load Loader, execute Executor, theme terminal.Theme, stderr io.Writer) (*Server, error) {
	s := &Server{
		dial:    dial,
		dialAPI: dialAPI,
		load:    load,
		execute: execute,
		server: sdk.NewServer(
			&sdk.Implementation{Name: buildinfo.AppName, Version: buildinfo.Version},
			&sdk.ServerOptions{Instructions: instructions, Capabilities: &sdk.ServerCapabilities{}},
		),
		console: &console{theme: theme, stderr: stderr},
	}

	if err := s.register(); err != nil {
		return nil, fmt.Errorf("failed to register the MCP tools: %w", err)
	}

	s.server.AddReceivingMiddleware(s.logCalls)

	return s, nil
}

// Run enters the project root, so the default config lookup reads it, writes the banner, then serves stdin and stdout
func (s *Server) Run(ctx context.Context) (int, error) {
	if err := os.Chdir(root()); err != nil {
		return 1, fmt.Errorf("failed to enter the project root: %w", err)
	}

	s.banner()

	err := s.server.Run(ctx, &sdk.StdioTransport{})
	if err != nil && !errors.Is(err, context.Canceled) {
		return 1, fmt.Errorf("failed to serve MCP: %w", err)
	}

	return 0, nil
}
