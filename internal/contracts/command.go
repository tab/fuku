package contracts

// Service command types (the payload of a service command is the model.Service it targets)
const (
	CommandStartService   MessageType = "cmd_start_service"
	CommandStopService    MessageType = "cmd_stop_service"
	CommandRestartService MessageType = "cmd_restart_service"
	CommandStopAll        MessageType = "cmd_stop_all"
)

// Action names a service control action
type Action string

// Action values
const (
	ActionStart   Action = "start"
	ActionStop    Action = "stop"
	ActionRestart Action = "restart"
)

// Command returns the service command that carries the action
func (a Action) Command() MessageType {
	switch a {
	case ActionStart:
		return CommandStartService
	case ActionStop:
		return CommandStopService
	default:
		return CommandRestartService
	}
}
