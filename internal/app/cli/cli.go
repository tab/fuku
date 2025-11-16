package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"fuku/internal/app/generator"
	"fuku/internal/app/runner"
	"fuku/internal/app/ui/wire"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

const (
	Usage = `Usage:
  fuku init [--profile=<NAME>] [--service=<NAME>] [--force] [--dry-run]
                                  Generate fuku.yaml from template
  fuku --run=<PROFILE>            Run services with specified profile (with TUI)
  fuku --run=<PROFILE> --no-ui    Run services without TUI
  fuku help                       Show help
  fuku version                    Show version

Examples:
  fuku init                       Generate fuku.yaml with defaults
  fuku init --profile=dev --service=backend
                                  Generate with custom profile and service
  fuku init --force               Overwrite existing fuku.yaml
  fuku init --dry-run             Preview generated file without writing
  fuku --run=default              Run all services with TUI
  fuku --run=core --no-ui         Run core services without TUI
  fuku --run=minimal              Run minimal services with TUI

TUI Controls (Services View):
  ↑/↓ or k/j                      Navigate services
  pgup/pgdn/home/end              Scroll viewport
  r                               Restart selected service
  s                               Stop/start selected service
  space                           Toggle logs for selected service
  l                               Switch to logs view
  q                               Quit (stops all services)

TUI Controls (Logs View):
  ↑/↓ or k/j                      Scroll logs
  pgup/pgdn/home/end              Scroll viewport
  l                               Switch back to services view
  q                               Quit (stops all services)`
)

// CLI defines the interface for cli operations
type CLI interface {
	// Run processes command-line arguments and returns an exit code and error
	Run(args []string) (exitCode int, err error)
}

// cli represents the command-line interface for the application
type cli struct {
	cfg       *config.Config
	runner    runner.Runner
	ui        wire.UI
	generator generator.Generator
	log       logger.Logger
}

// NewCLI creates a new cli instance
func NewCLI(
	cfg *config.Config,
	runner runner.Runner,
	ui wire.UI,
	generator generator.Generator,
	log logger.Logger,
) CLI {
	return &cli{
		cfg:       cfg,
		runner:    runner,
		ui:        ui,
		generator: generator,
		log:       log,
	}
}

// Run processes command-line arguments and executes commands
func (c *cli) Run(args []string) (int, error) {
	noUI := false
	force := false
	dryRun := false
	profile := config.DefaultProfile
	initProfile := ""
	initService := ""

	var remainingArgs []string

	for _, arg := range args {
		switch {
		case arg == "--no-ui":
			noUI = true
		case arg == "--force":
			force = true
		case arg == "--dry-run":
			dryRun = true
		case strings.HasPrefix(arg, "--run="):
			profile = strings.TrimPrefix(arg, "--run=")
			if profile == "" {
				profile = config.DefaultProfile
			}
		case strings.HasPrefix(arg, "--profile="):
			initProfile = strings.TrimPrefix(arg, "--profile=")
		case strings.HasPrefix(arg, "--service="):
			initService = strings.TrimPrefix(arg, "--service=")
		default:
			remainingArgs = append(remainingArgs, arg)
		}
	}

	if len(remainingArgs) == 0 {
		return c.handleRun(profile, noUI)
	}

	cmd := remainingArgs[0]

	switch cmd {
	case "init", "--init", "-i":
		return c.handleInit(initProfile, initService, force, dryRun)
	case "help", "--help", "-h":
		return c.handleHelp()
	case "version", "--version", "-v":
		return c.handleVersion()
	case "run", "-r":
		if len(remainingArgs) > 1 {
			profile = remainingArgs[1]
		}

		return c.handleRun(profile, noUI)
	default:
		return c.handleUnknown()
	}
}

// handleRun executes the run command with the specified profile
func (c *cli) handleRun(profile string, noUI bool) (int, error) {
	c.log.Debug().Msgf("Running with profile: %s", profile)

	ctx := context.Background()

	if noUI {
		if err := c.runner.Run(ctx, profile); err != nil {
			c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
			fmt.Printf("Error: %v\n", err)

			return 1, err
		}

		return 0, nil
	}

	p, err := c.ui(ctx, profile)
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to create UI")
		fmt.Fprintf(os.Stderr, "Failed to create UI: %v\n", err)

		return 1, err
	}

	errChan := make(chan error, 1)

	go func() {
		errChan <- c.runner.Run(ctx, profile)
	}()

	if _, err := p.Run(); err != nil {
		c.log.Error().Err(err).Msg("UI error")
		fmt.Fprintf(os.Stderr, "UI error: %v\n", err)

		return 1, err
	}

	if err := <-errChan; err != nil {
		c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
		fmt.Printf("Error: %v\n", err)

		return 1, err
	}

	return 0, nil
}

// handleHelp displays help information
func (c *cli) handleHelp() (int, error) {
	c.log.Debug().Msg("Displaying help information")
	fmt.Println(Usage)

	return 0, nil
}

// handleVersion displays version information
func (c *cli) handleVersion() (int, error) {
	c.log.Debug().Msg("Displaying version information")
	fmt.Printf("Version: %s\n", config.Version)

	return 0, nil
}

// handleInit generates a fuku.yaml file from template
func (c *cli) handleInit(profileName, serviceName string, force bool, dryRun bool) (int, error) {
	opts := generator.DefaultOptions()
	if profileName != "" {
		opts.ProfileName = profileName
	}

	if serviceName != "" {
		opts.ServiceName = serviceName
	}

	c.log.Debug().Msgf("Generating fuku.yaml (profile=%s, service=%s, force=%v, dryRun=%v)", opts.ProfileName, opts.ServiceName, force, dryRun)

	if err := c.generator.Generate(opts, force, dryRun); err != nil {
		c.log.Error().Err(err).Msg("Failed to generate fuku.yaml")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1, err
	}

	if !dryRun {
		fmt.Println("Generated fuku.yaml")
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Edit fuku.yaml to configure your services")
		fmt.Printf("  2. Run 'fuku --run=%s' to start services\n", opts.ProfileName)
	}

	return 0, nil
}

// handleUnknown handles unknown commands
func (c *cli) handleUnknown() (int, error) {
	c.log.Debug().Msg("Unknown command")
	fmt.Println("Unknown command. Use 'fuku help' for more information")

	return 1, nil
}
