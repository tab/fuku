package contracts

// EventCommandStarted carries a CommandStarted
const EventCommandStarted MessageType = "command_started"

// CommandStarted indicates a CLI command has begun execution
type CommandStarted struct {
	Command string
	Profile string
	UI      bool
}
