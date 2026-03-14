package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"fuku/internal/app/errors"
	"fuku/internal/config"
	"fuku/internal/config/template"
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
  fuku logs --profile <name> [service...] Stream logs from specific profile

  fuku --config <path>            Use custom config file (--config, -c)

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
  fuku -c custom.yaml run core    Use custom config file
  fuku --config /path/fuku.yaml   Use config from another directory`
)

// CLI handles standalone commands that run without config or FX container
type CLI struct {
	cmd *Options
}

// NewCLI creates a new CLI for standalone command execution
func NewCLI(cmd *Options) *CLI {
	return &CLI{cmd: cmd}
}

// Run executes the standalone command and returns the exit code
func (c *CLI) Run() int {
	switch c.cmd.Type {
	case CommandVersion:
		fmt.Printf("Version: %s\n", config.Version)
		return 0
	case CommandHelp:
		fmt.Println(Usage)
		return 0
	case CommandInit:
		exitCode, err := GenerateConfigFile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

		return exitCode
	default:
		return 0
	}
}

// ChangeToConfigDir changes to the config file's parent directory if it has path components
func ChangeToConfigDir(cmd *Options) error {
	if cmd.ConfigFile == "" {
		return nil
	}

	dir := filepath.Dir(cmd.ConfigFile)
	if dir == "." {
		return nil
	}

	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("%w: %w", errors.ErrFailedToReadConfig, err)
	}

	cmd.ConfigFile = filepath.Base(cmd.ConfigFile)

	return nil
}

// GenerateConfigFile creates a fuku.yaml template in the current directory
func GenerateConfigFile() (int, error) {
	for _, f := range []string{config.ConfigFile, config.ConfigFileAlt} {
		_, err := os.Stat(f)

		switch {
		case err == nil:
			fmt.Printf("%s already exists\n", f)
			return 0, nil
		case os.IsNotExist(err):
			continue
		default:
			return 1, fmt.Errorf("failed to check %s: %w", f, err)
		}
	}

	f, err := os.OpenFile(config.ConfigFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return 1, fmt.Errorf("failed to write %s: %w", config.ConfigFile, err)
	}

	defer f.Close()

	if _, err := f.Write(template.Content); err != nil {
		return 1, fmt.Errorf("failed to write %s: %w", config.ConfigFile, err)
	}

	fmt.Printf("Created %s\n", config.ConfigFile)

	return 0, nil
}
