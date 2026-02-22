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
	CommandStop
	CommandInit
	CommandLogs
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

// rootFlags holds flag values for the root command
type rootFlags struct {
	version bool
	run     string
	stop    string
	logs    bool
	init    bool
}

// Parse parses command-line args and returns a Options struct
func Parse(args []string) (*Options, error) {
	result := &Options{
		Type:    CommandRun,
		Profile: config.Default,
	}

	var flags rootFlags

	root := buildRootCommand(result, &flags)
	root.AddCommand(
		buildInitCommand(result),
		buildRunCommand(result),
		buildStopCommand(result),
		buildLogsCommand(result),
		buildVersionCommand(result),
	)

	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		return nil, err
	}

	if flags.version {
		result.Type = CommandVersion
	}

	if flags.run != "" {
		result.Type = CommandRun
		result.Profile = flags.run
	}

	if flags.stop != "" {
		result.Type = CommandStop
		result.Profile = flags.stop
	}

	if flags.logs {
		result.Type = CommandLogs
		result.Profile = ""
		result.Services = []string{}
	}

	if flags.init {
		result.Type = CommandInit
	}

	return result, nil
}

// buildRootCommand creates the root cobra command
func buildRootCommand(result *Options, flags *rootFlags) *cobra.Command {
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
	cmd.Flags().BoolVarP(&flags.version, "version", "v", false, "Show version information")
	cmd.Flags().StringVarP(&flags.run, "run", "r", "", "Run services with specified profile")
	cmd.Flags().StringVarP(&flags.stop, "stop", "s", "", "Stop services with specified profile")
	cmd.Flags().BoolVarP(&flags.logs, "logs", "l", false, "Stream logs from running services")
	cmd.Flags().BoolVarP(&flags.init, "init", "i", false, "Generate fuku.yaml template")

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		result.Type = CommandHelp
	})

	return cmd
}

// buildInitCommand creates the init subcommand
func buildInitCommand(result *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"i"},
		Short:   "Generate fuku.yaml template",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			result.Type = CommandInit
		},
	}

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

// buildStopCommand creates the stop subcommand
func buildStopCommand(result *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stop [profile]",
		Aliases: []string{"s"},
		Short:   "Stop services by killing processes in service directories",
		Args:    cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			result.Type = CommandStop
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

// buildVersionCommand creates the version subcommand
func buildVersionCommand(result *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			result.Type = CommandVersion
		},
	}

	return cmd
}
