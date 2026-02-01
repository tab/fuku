package services

import (
	"context"

	"fuku/internal/app/bus"
)

// Controller handles business logic for service orchestration
type Controller interface {
	Start(ctx context.Context, service *ServiceState)
	Stop(ctx context.Context, service *ServiceState)
	Restart(ctx context.Context, service *ServiceState)
	StopAll()
	HandleStarting(ctx context.Context, service *ServiceState, pid int)
	HandleReady(ctx context.Context, service *ServiceState)
	HandleFailed(ctx context.Context, service *ServiceState)
	HandleStopping(ctx context.Context, service *ServiceState)
	HandleStopped(ctx context.Context, service *ServiceState) bool
	HandleRestarting(ctx context.Context, service *ServiceState)
}

// controller implements the Controller interface
type controller struct {
	bus bus.Bus
}

// NewController creates a new controller with the given bus
func NewController(b bus.Bus) Controller {
	return &controller{
		bus: b,
	}
}

// Start requests a service start if it's currently stopped
func (c *controller) Start(ctx context.Context, service *ServiceState) {
	if service.IsNil() {
		return
	}

	if service.FSM.Current() != Stopped && service.FSM.Current() != Failed {
		return
	}

	c.bus.Publish(bus.Message{
		Type: bus.CommandRestartService,
		Data: bus.Payload{Name: service.Name},
	})
}

// Stop requests a service stop if it's currently running
func (c *controller) Stop(ctx context.Context, service *ServiceState) {
	if service.IsNil() {
		return
	}

	if service.FSM.Current() != Running {
		return
	}

	c.bus.Publish(bus.Message{
		Type: bus.CommandStopService,
		Data: bus.Payload{Name: service.Name},
	})
}

// Restart requests a service restart if it's running, failed, or stopped
func (c *controller) Restart(ctx context.Context, service *ServiceState) {
	if service.IsNil() {
		return
	}

	state := service.FSM.Current()
	if state != Running && state != Failed && state != Stopped {
		return
	}

	// Optimistic UI update - immediately transition to Restarting for instant feedback
	_ = service.FSM.Event(ctx, Restart)

	c.bus.Publish(bus.Message{
		Type: bus.CommandRestartService,
		Data: bus.Payload{Name: service.Name},
	})
}

// StopAll sends a command to stop all services
func (c *controller) StopAll() {
	c.bus.Publish(bus.Message{Type: bus.CommandStopAll})
}

// HandleStarting updates service state when a process starts
func (c *controller) HandleStarting(ctx context.Context, service *ServiceState, pid int) {
	if service == nil {
		return
	}

	service.Monitor.PID = pid
	if service.FSM != nil {
		_ = service.FSM.Event(ctx, Start)
	}
}

// HandleReady updates service state when it becomes ready
func (c *controller) HandleReady(ctx context.Context, service *ServiceState) {
	if service == nil {
		return
	}

	if service.FSM != nil {
		_ = service.FSM.Event(ctx, Started)
	}
}

// HandleFailed updates service state when it fails
func (c *controller) HandleFailed(ctx context.Context, service *ServiceState) {
	if service == nil {
		return
	}

	if service.FSM != nil {
		_ = service.FSM.Event(ctx, Failed)
	}
}

// HandleStopping updates service state when it begins stopping
func (c *controller) HandleStopping(ctx context.Context, service *ServiceState) {
	if service == nil {
		return
	}

	if service.FSM != nil {
		_ = service.FSM.Event(ctx, Stop)
	}
}

// HandleRestarting updates service state when it begins restarting
func (c *controller) HandleRestarting(ctx context.Context, service *ServiceState) {
	if service == nil {
		return
	}

	// Skip if already in Restarting state (optimistic update already applied)
	if service.FSM != nil && service.FSM.Current() != Restarting {
		_ = service.FSM.Event(ctx, Restart)
	}
}

// HandleStopped updates service state when it stops, returns true if it was restarting
func (c *controller) HandleStopped(ctx context.Context, service *ServiceState) bool {
	if service == nil {
		return false
	}

	wasRestarting := false
	if service.FSM != nil {
		wasRestarting = service.FSM.Current() == Restarting

		err := service.FSM.Event(ctx, Stopped)
		if err != nil {
			service.MarkStopped()
		}
	} else {
		service.MarkStopped()
	}

	return wasRestarting
}
