package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"fuku/internal/app/errors"
	"fuku/internal/app/relay"
	"fuku/internal/config"
)

// CommandType represents the type of CLI command
type CommandType string

// Command type values
const (
	CommandRun     CommandType = "run"
	CommandStop    CommandType = "stop"
	CommandInit    CommandType = "init"
	CommandLogs    CommandType = "logs"
	CommandVersion CommandType = "version"
	CommandHelp    CommandType = "help"
	CommandDoctor  CommandType = "doctor"
)

// Standalone returns true for commands that run without config or FX container
func (c CommandType) Standalone() bool {
	switch c {
	case CommandInit, CommandVersion, CommandHelp:
		return true
	default:
		return false
	}
}

// RequiresServices returns true for commands that need at least one service defined in the config
func (c CommandType) RequiresServices() bool {
	switch c {
	case CommandRun, CommandStop:
		return true
	default:
		return false
	}
}

// String returns the string representation of a CommandType
func (c CommandType) String() string {
	return string(c)
}

// Flag represents the name of a CLI flag
type Flag string

// Flag name values
const (
	FlagConfig   Flag = "config"
	FlagNoUI     Flag = "no-ui"
	FlagProfile  Flag = "profile"
	FlagTail     Flag = "tail"
	FlagNoFollow Flag = "no-follow"
	FlagSummary  Flag = "summary"
	FlagJSON     Flag = "json"
)

// String returns the string representation of a Flag
func (f Flag) String() string {
	return string(f)
}

// DoctorFormat selects the doctor renderer
type DoctorFormat int

// DoctorFormat values
const (
	DoctorFormatText DoctorFormat = iota
	DoctorFormatSummary
	DoctorFormatJSON
)

// Options contains the parsed command-line arguments
type Options struct {
	ConfigFile   string
	Type         CommandType
	Profile      string
	Services     []string
	NoUI         bool
	DoctorFormat DoctorFormat
	relay.ReplayOptions
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
		buildDoctorCommand(result),
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

	if result.ConfigFile != "" && result.Type.Standalone() {
		return nil, errors.ErrConfigFlagNotSupported
	}

	return result, nil
}

// buildRootCommand creates the root cobra command
func buildRootCommand(result *Options, flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:           config.AppName,
		Short:         "A lightweight CLI orchestrator for running and managing multiple local services",
		Long:          "Fuku is a lightweight CLI orchestrator for running and managing multiple local services in development environments",
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
			result.Type = CommandRun
		},
	}

	cmd.PersistentFlags().BoolVar(&result.NoUI, FlagNoUI.String(), false, "Run without TUI")
	cmd.PersistentFlags().StringVarP(&result.ConfigFile, FlagConfig.String(), "c", "", "Path to config file (disables override merging)")
	cmd.Flags().BoolVarP(&flags.version, CommandVersion.String(), "v", false, "Show version information")
	cmd.Flags().StringVarP(&flags.run, CommandRun.String(), "r", "", "Run services with specified profile")
	cmd.Flags().StringVarP(&flags.stop, CommandStop.String(), "s", "", "Stop services with specified profile")
	cmd.Flags().BoolVarP(&flags.logs, CommandLogs.String(), "l", false, "Stream logs from running services")
	cmd.Flags().BoolVarP(&flags.init, CommandInit.String(), "i", false, "Generate fuku.yaml template")

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		result.Type = CommandHelp
	})

	return cmd
}

// buildInitCommand creates the init subcommand
func buildInitCommand(result *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     CommandInit.String(),
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
		Use:     CommandRun.String() + " [profile]",
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
		Use:     CommandStop.String() + " [profile]",
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
	var (
		logsProfile string
		logsReplay  relay.ReplayOptions
	)

	cmd := &cobra.Command{
		Use:     CommandLogs.String() + " [services...]",
		Aliases: []string{"l"},
		Short:   "Stream logs from running services",
		Run: func(cmd *cobra.Command, args []string) {
			result.Type = CommandLogs
			result.Services = args
			result.Profile = logsProfile
			result.ReplayOptions = logsReplay
		},
	}

	cmd.Flags().StringVar(&logsProfile, FlagProfile.String(), "", "Filter by profile")
	cmd.Flags().Var(&tailValue{target: &logsReplay.Tail}, FlagTail.String(), "Replay at most the newest n buffered messages")
	cmd.Flags().BoolVar(&logsReplay.NoFollow, FlagNoFollow.String(), false, "Exit after the buffered replay instead of following")

	return cmd
}

// buildVersionCommand creates the version subcommand
func buildVersionCommand(result *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   CommandVersion.String(),
		Short: "Show version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			result.Type = CommandVersion
		},
	}

	return cmd
}

// buildDoctorCommand creates the doctor subcommand
func buildDoctorCommand(result *Options) *cobra.Command {
	var (
		summary bool
		asJSON  bool
	)

	cmd := &cobra.Command{
		Use:   CommandDoctor.String() + " [profile]",
		Short: "Diagnose configuration, environment, and runtime issues",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			result.Type = CommandDoctor

			if len(args) > 0 {
				result.Profile = args[0]
			}

			switch {
			case asJSON:
				result.DoctorFormat = DoctorFormatJSON
			case summary:
				result.DoctorFormat = DoctorFormatSummary
			default:
				result.DoctorFormat = DoctorFormatText
			}
		},
	}

	cmd.Flags().BoolVar(&summary, FlagSummary.String(), false, "Print a compact one-line-per-check report")
	cmd.Flags().BoolVar(&asJSON, FlagJSON.String(), false, "Print the report as JSON")

	return cmd
}

// tailValue parses the --tail flag into an optional positive integer (nil until the flag is provided)
type tailValue struct {
	target **int
}

// String returns the parsed value, or an empty string before the flag is provided
func (v *tailValue) String() string {
	if *v.target == nil {
		return ""
	}

	return strconv.Itoa(**v.target)
}

// Set parses one --tail argument and rejects values that are not greater than zero
func (v *tailValue) Set(raw string) error {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return err
	}

	if n <= 0 {
		return errors.ErrInvalidTail
	}

	*v.target = &n

	return nil
}

// Type names the flag value in usage output
func (v *tailValue) Type() string {
	return "int"
}
