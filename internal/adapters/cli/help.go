package cli

import (
	"context"
	"fmt"
	"io"
)

// Help text constants
const (
	Usage = `Usage:
  fuku                            Run services with default profile (with TUI)

  fuku init                       Generate fuku.yaml template (--init, -i, init, i)

  fuku run <profile>              Run services with specified profile
  fuku --run <profile>            Same as above (--run, -r, run, r)
  fuku run <profile> --no-ui      Run services without TUI

  fuku stop                       Stop services with default profile
  fuku stop <profile>             Stop services with specified profile
  fuku --stop <profile>           Same as above (--stop, -s, stop, s)

  fuku logs [service...]          Stream logs from running services
  fuku --logs                     Same as above (--logs, -l, logs, l)
  fuku logs api --profile <name>  Fail unless the instance runs profile <name>
  fuku logs api --tail <n>        Replay at most the newest n messages
  fuku logs api --no-follow       Exit after the buffered replay
  fuku logs api --no-ui           Hide the logs panel and footer

  fuku doctor [profile]           Diagnose configuration, environment, and runtime issues
  fuku doctor --summary           Print a compact one-line-per-check report
  fuku doctor --json              Print the report as JSON

  fuku --config <path>            Use custom config file, skip override merging (--config, -c)

  fuku help                       Show help (--help, -h, help)
  fuku version                    Show version (--version, -v, version)

Examples:
  fuku                            Run default profile with TUI
  fuku init                       Generate fuku.yaml in current directory
  fuku run core --no-ui           Run core services without TUI
  fuku -r core --no-ui            Same as above using flag
  fuku stop                       Stop all services (default profile)
  fuku stop backend               Stop backend services
  fuku logs                       Stream all logs from running fuku
  fuku logs api auth              Stream logs from api and auth services
  fuku -l                         Stream logs using flag
  fuku -c custom.yaml run core    Use custom config file (no override merging)
  fuku --config /path/fuku.yaml   Use config from another directory (no override merging)`
)

// Help prints the usage text
type Help struct {
	stdout io.Writer
}

// NewHelp creates the help command
func NewHelp(stdout io.Writer) *Help {
	return &Help{stdout: stdout}
}

// Run prints the usage and returns the exit code
func (h *Help) Run(context.Context) (int, error) {
	fmt.Fprintln(h.stdout, Usage)

	return 0, nil
}
