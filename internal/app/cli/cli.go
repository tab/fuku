//go:generate mockgen -source=cli.go -destination=cli_mock.go -package=cli
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"fuku/internal/app/colors"
	"fuku/internal/app/errors"
	"fuku/internal/app/runner"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

const (
	appName = "fuku"
	appDesc = "lightweight CLI orchestrator for running services in development environment"
	help    = "Show help information"
	version = "Show version information"
	run     = "Run services using specified profile"
)

// CLI defines the interface for cli operations
type CLI interface {
	Run(args []string) error
}

// PhaseTracker tracks execution phases for UI display
type PhaseTracker struct {
	currentPhase string
	startTime    time.Time
	phases       map[string]time.Duration
}

// NewPhaseTracker creates a new phase tracker
func NewPhaseTracker() *PhaseTracker {
	return &PhaseTracker{
		phases: make(map[string]time.Duration),
	}
}

// StartPhase begins tracking a new phase
func (pt *PhaseTracker) StartPhase(phase string) {
	if pt.currentPhase != "" {
		pt.phases[pt.currentPhase] = time.Since(pt.startTime)
	}
	pt.currentPhase = phase
	pt.startTime = time.Now()
}

// GetCurrentPhase returns the current phase
func (pt *PhaseTracker) GetCurrentPhase() string {
	return pt.currentPhase
}

// GetPhaseDuration returns the duration of a completed phase
func (pt *PhaseTracker) GetPhaseDuration(phase string) time.Duration {
	return pt.phases[phase]
}

// GetAllPhases returns all phases with their durations
func (pt *PhaseTracker) GetAllPhases() map[string]time.Duration {
	result := make(map[string]time.Duration)
	for k, v := range pt.phases {
		result[k] = v
	}
	if pt.currentPhase != "" {
		result[pt.currentPhase] = time.Since(pt.startTime)
	}
	return result
}

// cli represents the command-line interface for the application
type cli struct {
	cfg    *config.Config
	runner runner.Runner
	phases *PhaseTracker
	log    logger.Logger
}

// NewCLI creates a new cli instance
func NewCLI(
	cfg *config.Config,
	runner runner.Runner,
	log logger.Logger,
) CLI {
	return &cli{
		cfg:    cfg,
		runner: runner,
		phases: NewPhaseTracker(),
		log:    log,
	}
}

// Run processes command-line arguments and executes commands
func (c *cli) Run(args []string) error {
	if len(args) == 0 {
		return c.handleRun(config.DefaultProfile)
	}

	cmd := args[0]

	switch {
	case c.isHelpCommand(cmd):
		return c.handleHelp()
	case c.isVersionCommand(cmd):
		return c.handleVersion()
	case c.isRunCommand(cmd):
		return c.handleRunCommand(args)
	default:
		return c.handleUnknown()
	}
}

func (c *cli) isHelpCommand(cmd string) bool {
	return cmd == "help" || cmd == "--help" || cmd == "-h"
}

func (c *cli) isVersionCommand(cmd string) bool {
	return cmd == "version" || cmd == "--version" || cmd == "-v"
}

func (c *cli) isRunCommand(cmd string) bool {
	return cmd == "run" || cmd == "--run" || cmd == "-r" || strings.HasPrefix(cmd, "--run=")
}

// Handle run command with profile parsing
func (c *cli) handleRunCommand(args []string) error {
	cmd := args[0]
	profile := config.DefaultProfile

	switch {
	case strings.HasPrefix(cmd, "--run="):
		profile = strings.TrimPrefix(cmd, "--run=")
		if profile == "" {
			profile = config.DefaultProfile
		}
	case (cmd == "run" || cmd == "--run" || cmd == "-r") && len(args) > 1:
		profile = args[1]
	}

	return c.handleRun(profile)
}

// handleRun executes the run command with the specified profile
func (c *cli) handleRun(profile string) error {
	c.log.Debug().Msgf("Running with profile: %s", profile)
	c.phases.StartPhase("Discovery")

	ctx := context.Background()
	if err := c.runner.Run(ctx, profile); err != nil {
		c.log.Error().Err(err).Msgf("Failed to run profile '%s'", profile)
		fmt.Fprintf(os.Stderr, "%s %v\n", colors.Error("Error:"), err)
		return err
	}

	return nil
}

// handleHelp displays help information
func (c *cli) handleHelp() error {
	c.log.Debug().Msg("Displaying help information")
	c.printHelp()
	return nil
}

// printHelp prints formatted help information
func (c *cli) printHelp() {
	fmt.Printf("\n%s %s\n", colors.Title(appName), colors.Success("v"+config.Version))
	fmt.Printf("%s\n\n", colors.Muted(appDesc))

	fmt.Printf("%s\n", colors.Subtitle("USAGE"))
	fmt.Printf("  %s %s\n\n", appName, colors.Muted("[command] [options]"))

	fmt.Printf("%s\n", colors.Subtitle("COMMANDS"))
	fmt.Printf("  %-12s %s\n", colors.Primary("help"), colors.Muted(help))
	fmt.Printf("  %-12s %s\n", colors.Primary("version"), colors.Muted(version))
	fmt.Printf("  %-12s %s\n\n", colors.Primary("run"), colors.Muted(run))

	fmt.Printf("%s\n", colors.Subtitle("OPTIONS"))
	fmt.Printf("  %-12s %s\n", colors.Primary("-h, --help"), colors.Muted("Show help information"))
	fmt.Printf("  %-12s %s\n", colors.Primary("-v, --version"), colors.Muted("Show version information"))
	fmt.Printf("  %-12s %s\n", colors.Primary("-r, --run"), colors.Muted("Run services with profile"))
	fmt.Printf("  %-12s %s\n\n", colors.Primary("--run=<profile>"), colors.Muted("Run specific profile"))

	fmt.Printf("%s\n", colors.Subtitle("EXAMPLES"))
	fmt.Printf("  %s %s %s\n", appName, colors.Success("--run=default"), colors.Muted("Run all services"))
	fmt.Printf("  %s %s %s\n", appName, colors.Success("--run=core"), colors.Muted("Run core services"))
	fmt.Printf("  %s %s %s\n\n", appName, colors.Success("version"), colors.Muted("Show version"))
}

// handleVersion displays version information
func (c *cli) handleVersion() error {
	c.log.Debug().Msg("Displaying version information")
	fmt.Printf("\n%s %s\n", colors.Title(appName), colors.Success("v"+config.Version))
	fmt.Printf("%s\n\n", colors.Muted(appDesc))
	return nil
}

// printPhases displays current execution phases with colors
func (c *cli) printPhases() {
	fmt.Printf("\n%s\n\n", colors.Title("Execution Phases"))

	phases := c.phases.GetAllPhases()
	currentPhase := c.phases.GetCurrentPhase()

	// Always show the three main phases
	mainPhases := []string{"Discovery", "Planning", "Execution"}

	for _, phase := range mainPhases {
		duration := phases[phase]
		isActive := phase == currentPhase

		var status, timing string
		if duration > 0 {
			if isActive {
				status = colors.StatusRunningColor(colors.StatusRunning)
				timing = fmt.Sprintf("Running for %s", duration.Truncate(time.Millisecond))
			} else {
				status = colors.StatusSuccessColor(colors.StatusSuccess)
				timing = fmt.Sprintf("Completed in %s", duration.Truncate(time.Millisecond))
			}
		} else {
			status = colors.StatusPendingColor(colors.StatusPending)
			timing = "Pending"
		}

		fmt.Printf("%s %s\n", status, colors.PhaseColor(phase, phase))
		fmt.Printf("  %s %s\n\n", colors.ProgressArrow, colors.Muted(timing))
	}
}

// handleUnknown handles unknown commands
func (c *cli) handleUnknown() error {
	c.log.Debug().Msg("Unknown command")
	fmt.Printf("\n%s Unknown command.\n", colors.Error("Error:"))
	fmt.Printf("Use '%s' for more information.\n\n", colors.Primary("fuku help"))
	return errors.ErrUnknownCommand
}
