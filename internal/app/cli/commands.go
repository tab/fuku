package cli

import (
	"github.com/spf13/cobra"

	"fuku/internal/config"
)

// CommandType represents the type of CLI command
type CommandType int

// Command type values
const (
	CommandRun CommandType = iota
	CommandLogs
	CommandStop
	CommandVersion
	CommandHelp
)

// Options contains the parsed command-line arguments
type Options struct {
	Type     CommandType
	Profile  string
	Services []string
	NoUI     bool
}

// Parse parses command-line args and returns a Options struct
func Parse(args []string) (*Options, error) {
	result := &Options{
		Type:    CommandRun,
		Profile: config.Default,
	}

	var (
		showVersion bool
		runProfile  string
		logsFlag    bool
	)

	root := buildRootCommand(result, &showVersion, &runProfile, &logsFlag)
	root.AddCommand(
		buildRunCommand(result),
		buildLogsCommand(result),
		buildStopCommand(result),
		buildVersionCommand(result),
	)

	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		return nil, err
	}

	if showVersion {
		result.Type = CommandVersion
	}

	if runProfile != "" {
		result.Type = CommandRun
		result.Profile = runProfile
	}

	if logsFlag {
		result.Type = CommandLogs
		result.Profile = ""
		result.Services = []string{}
	}

	return result, nil
}

// buildRootCommand creates the root cobra command
func buildRootCommand(result *Options, showVersion *bool, runProfile *string, logsFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fuku",
		Short: "A lightweight CLI orchestrator for running and managing multiple local services",
		Long: `Fuku is a lightweight CLI orchestrator for running and managing
multiple local services in development environments.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
			result.Type = CommandRun
		},
	}

	cmd.PersistentFlags().BoolVar(&result.NoUI, "no-ui", false, "Run without TUI")
	cmd.Flags().BoolVarP(showVersion, "version", "v", false, "Show version information")
	cmd.Flags().StringVarP(runProfile, "run", "r", "", "Run services with specified profile")
	cmd.Flags().BoolVarP(logsFlag, "logs", "l", false, "Stream logs from running services")

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		result.Type = CommandHelp
	})

	return cmd
}

// buildRunCommand creates the run subcommand
func buildRunCommand(result *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run [profile]",
		Aliases: []string{"r"},
		Short:   "Run services with the specified profile",
		Args:    cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			result.Type = CommandRun
			if len(args) > 0 {
				result.Profile = args[0]
			}
		},
	}

	return cmd
}

// buildLogsCommand creates the logs subcommand
func buildLogsCommand(result *Options) *cobra.Command {
	var logsProfile string

	cmd := &cobra.Command{
		Use:     "logs [services...]",
		Aliases: []string{"l"},
		Short:   "Stream logs from running services",
		Run: func(cmd *cobra.Command, args []string) {
			result.Type = CommandLogs
			result.Services = args
			result.Profile = logsProfile
		},
	}

	cmd.Flags().StringVar(&logsProfile, "profile", "", "Filter by profile")

	return cmd
}

// buildStopCommand creates the stop subcommand
func buildStopCommand(result *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stop",
		Aliases: []string{"s"},
		Short:   "Stop orphaned processes from a previous session",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			result.Type = CommandStop
		},
	}

	return cmd
}

// buildVersionCommand creates the version subcommand
func buildVersionCommand(result *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			result.Type = CommandVersion
		},
	}

	return cmd
}
