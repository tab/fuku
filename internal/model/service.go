package model

import "time"

// Service is one service of the project and its runtime state
type Service struct {
	ID          string
	Name        string
	Command     string
	Directory   string
	Tier        string
	Readiness   *Readiness
	Watch       *Watch
	Environment *EnvFiles
	LogOutput   []string
	Status      Status
	Watching    bool
	Error       string
	Process     Process
	AttemptedAt time.Time
	LifecycleAt time.Time
}

// Process is the child the service runs right now (the zero value means none)
type Process struct {
	PID       int
	CPU       float64
	Memory    uint64
	StartedAt time.Time
}

// SettledSince returns true if an event at or after since ended the action, and a watched service is watching
func (s *Service) SettledSince(since time.Time) bool {
	if !s.Status.IsSettled() || s.LifecycleAt.Before(since) {
		return false
	}

	return s.Status != StatusRunning || s.Watch == nil || s.Watching
}
