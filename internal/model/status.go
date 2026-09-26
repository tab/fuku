package model

// Status represents the lifecycle status of a service
type Status string

// Status values for service lifecycle
const (
	StatusPending    Status = "pending"
	StatusStarting   Status = "starting"
	StatusRunning    Status = "running"
	StatusStopping   Status = "stopping"
	StatusRestarting Status = "restarting"
	StatusFailed     Status = "failed"
	StatusStopped    Status = "stopped"
)

// IsRunning returns true if the status is running
func (s Status) IsRunning() bool {
	return s == StatusRunning
}
