package contracts

import "time"

// Preflight event types
const (
	EventPreflightStarted  MessageType = "preflight_started"
	EventPreflightKilled   MessageType = "preflight_kill" // frozen wire value, log consumers read it
	EventPreflightComplete MessageType = "preflight_complete"
)

// PreflightStarted indicates the preflight scan has begun
type PreflightStarted struct {
	Services []string
}

// PreflightKilled indicates a process was killed during preflight
type PreflightKilled struct {
	Service string
	Name    string
	PID     int
}

// PreflightComplete indicates the preflight scan has finished
type PreflightComplete struct {
	Killed   int
	Duration time.Duration
}
