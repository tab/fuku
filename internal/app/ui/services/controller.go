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

type controller struct {
	command runtime.CommandBus
}

// NewController creates a new service controller
func NewController(command runtime.CommandBus) Controller {
	return &controller{
		command: command,
	}
}

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

func (c *controller) StopAll() {
	c.command.Publish(runtime.Command{Type: runtime.CommandStopAll})
}

func (c *controller) HandleStarting(ctx context.Context, service *ServiceState, pid int) {
	if service == nil {
		return
	}

	service.Monitor.PID = pid
	if service.FSM != nil {
		_ = service.FSM.Event(ctx, Start)
	}
}

func (c *controller) HandleReady(ctx context.Context, service *ServiceState) {
	if service == nil {
		return
	}

	if service.FSM != nil {
		_ = service.FSM.Event(ctx, Started)
	}
}

func (c *controller) HandleFailed(ctx context.Context, service *ServiceState) {
	if service == nil {
		return
	}

	if service.FSM != nil {
		_ = service.FSM.Event(ctx, Failed)
	}
}

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
