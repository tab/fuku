package runner

import (
	"time"
)

// EventType represents different types of runner events
type EventType string

const (
	EventPhaseStart   EventType = "phase_start"
	EventPhaseDone    EventType = "phase_done"
	EventServiceStart EventType = "service_start"
	EventServiceReady EventType = "service_ready"
	EventServiceLog   EventType = "service_log"
	EventServiceStop  EventType = "service_stop"
	EventServiceFail  EventType = "service_fail"
	EventReady        EventType = "ready"
	EventError        EventType = "error"
)

// Phase represents different execution phases
type Phase string

const (
	PhaseDiscovery Phase = "discovery"
	PhaseExecution Phase = "execution"
	PhaseRunning   Phase = "running"
	PhaseShutdown  Phase = "shutdown"
)

// Event represents a runner event with associated data
type Event struct {
	Type      EventType
	Timestamp time.Time
	Data      interface{}
}

// PhaseStart indicates a phase has started
type PhaseStart struct {
	Phase Phase
}

// PhaseDone indicates a phase has completed
type PhaseDone struct {
	Phase        Phase
	ServiceCount int      // Total number of services discovered (for discovery phase)
	ServiceNames []string // List of service names (for discovery phase)
}

// ServiceStart indicates a service has started
type ServiceStart struct {
	Name      string
	PID       int
	StartTime time.Time
}

// ServiceReady indicates a service has passed readiness checks
type ServiceReady struct {
	Name      string
	ReadyTime time.Time
}

// ServiceLog contains a log line from a service
type ServiceLog struct {
	Name   string
	Stream string
	Line   string
	Time   time.Time
}

// ServiceStop indicates a service has stopped
type ServiceStop struct {
	Name         string
	ExitCode     int
	StopTime     time.Time
	GracefulStop bool
}

// ServiceFail indicates a service has failed
type ServiceFail struct {
	Name  string
	Error error
	Time  time.Time
}

// ErrorData indicates an error occurred
type ErrorData struct {
	Error error
	Time  time.Time
}

// EventCallback is a function that receives runner events
type EventCallback func(Event)

// ServiceControl represents a service control action
type ServiceControl int

const (
	ControlStop ServiceControl = iota
	ControlStart
	ControlRestart
)

// ServiceControlRequest represents a request to control a service
type ServiceControlRequest struct {
	ServiceName string
	Action      ServiceControl
}
