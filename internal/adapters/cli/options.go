package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"fuku/internal/contracts"
	"fuku/internal/model"
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

// standalone returns true for commands that take no config file
func (c CommandType) standalone() bool {
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

// Format selects the doctor renderer
type Format int

// Format values
const (
	FormatText Format = iota
	FormatSummary
	FormatJSON
)

// Options contains the parsed command-line arguments
type Options struct {
	ConfigFile   string
	Type         CommandType
	Profile      string
	Services     []string
	NoUI         bool
	DoctorFormat Format
	model.ReplayOptions
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
		return fmt.Errorf("%w: %w", contracts.ErrFailedToReadConfig, err)
	}

	cmd.ConfigFile = filepath.Base(cmd.ConfigFile)

	return nil
}
