package services

import (
	"context"

	"fuku/internal/app/runtime"
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
	HandleStopped(ctx context.Context, service *ServiceState) bool
}

// controller implements the Controller interface
type controller struct {
	command runtime.CommandBus
}

// NewController creates a new controller with the given command bus
func NewController(command runtime.CommandBus) Controller {
	return &controller{
		command: command,
	}
}

// Start requests a service start if it's currently stopped
func (c *controller) Start(ctx context.Context, service *ServiceState) {
	if service == nil || service.FSM == nil {
		return
	}

	if service.FSM.Current() != Stopped {
		return
	}

	c.command.Publish(runtime.Command{
		Type: runtime.CommandRestartService,
		Data: runtime.RestartServiceData{Service: service.Name},
	})
	_ = service.FSM.Event(ctx, Start)
}

// Stop requests a service stop if it's currently running
func (c *controller) Stop(ctx context.Context, service *ServiceState) {
	if service == nil || service.FSM == nil {
		return
	}

	if service.FSM.Current() != Running {
		return
	}

	c.command.Publish(runtime.Command{
		Type: runtime.CommandStopService,
		Data: runtime.StopServiceData{Service: service.Name},
	})

	_ = service.FSM.Event(ctx, Stop)
}

// Restart requests a service restart if it's running, failed, or stopped
func (c *controller) Restart(ctx context.Context, service *ServiceState) {
	if service == nil || service.FSM == nil {
		return
	}

	state := service.FSM.Current()
	if state != Running && state != Failed && state != Stopped {
		return
	}

	c.command.Publish(runtime.Command{
		Type: runtime.CommandRestartService,
		Data: runtime.RestartServiceData{Service: service.Name},
	})

	_ = service.FSM.Event(ctx, Restart)
}

// StopAll sends a command to stop all services
func (c *controller) StopAll() {
	c.command.Publish(runtime.Command{Type: runtime.CommandStopAll})
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
