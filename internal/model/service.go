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
