package model

// Phase represents the application phase
type Phase string

// Phase values for the application lifecycle
const (
	PhaseStartup  Phase = "startup"
	PhaseRunning  Phase = "running"
	PhaseStopping Phase = "stopping"
	PhaseStopped  Phase = "stopped"
)
