package contracts

// EventCommandStarted carries a CommandStarted
const EventCommandStarted MessageType = "command_started"

// CommandNameRun is the CommandStarted.Command value for the run command
const CommandNameRun = "run"

// CommandStarted indicates a CLI command has begun execution
type CommandStarted struct {
	Command string
	Profile string
	UI      bool
}
