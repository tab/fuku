package contracts

import (
	"time"

	"fuku/internal/model"
)

// Service lifecycle event types
const (
	EventServiceStarting   MessageType = "service_starting"
	EventServiceReady      MessageType = "service_ready"
	EventServiceFailed     MessageType = "service_failed"
	EventServiceStopping   MessageType = "service_stopping"
	EventServiceStopped    MessageType = "service_stopped"
	EventServiceRestarting MessageType = "service_restarting"
)

// ServiceEvent is the base struct for service-related events
type ServiceEvent struct {
	Service model.Service
	Tier    string
}

// ServiceStarting indicates a service is starting with attempt and process info
type ServiceStarting struct {
	ServiceEvent
	Attempt   int
	PID       int
	StartedAt time.Time
}

// ServiceReady indicates a service has completed startup and is ready
type ServiceReady struct {
	ServiceEvent
	PID       int
	StartedAt time.Time
	Duration  time.Duration
}

// ServiceFailed indicates a service failed to start or crashed
type ServiceFailed struct {
	ServiceEvent
	Error error
}

// ServiceStopping indicates a service is being stopped
type ServiceStopping struct {
	ServiceEvent
}

// ServiceStopped indicates a service has stopped
type ServiceStopped struct {
	ServiceEvent
	Unexpected bool
}

// ServiceRestarting indicates a service is being restarted
type ServiceRestarting struct {
	ServiceEvent
}
