package services

import (
	"context"
	"fmt"

	"github.com/looplab/fsm"
)

// FSM states
const (
	Stopped    = "stopped"
	Starting   = "starting"
	Running    = "running"
	Stopping   = "stopping"
	Restarting = "restarting"
	Failed     = "failed"
)

// FSM events
const (
	Start   = "start"
	Stop    = "stop"
	Restart = "restart"
	Started = "started"
)

// FSM callbacks
const (
	OnStarting   = "enter_starting"
	OnStopping   = "enter_stopping"
	OnRestarting = "enter_restarting"
	OnRunning    = "enter_running"
	OnStopped    = "enter_stopped"
	OnFailed     = "enter_failed"
)

// newServiceFSM creates a state machine for service lifecycle management
func newServiceFSM(service *ServiceState, loader *Loader) *fsm.FSM {
	serviceName := service.Name

	return fsm.NewFSM(
		Stopped,
		fsm.Events{
			{Name: Start, Src: []string{Stopped, Restarting}, Dst: Starting},
			{Name: Stop, Src: []string{Running}, Dst: Stopping},
			{Name: Restart, Src: []string{Running, Failed, Stopped}, Dst: Restarting},
			{Name: Started, Src: []string{Starting}, Dst: Running},
			{Name: Stopped, Src: []string{Stopping, Restarting}, Dst: Stopped},
			{Name: Failed, Src: []string{Starting, Running, Restarting}, Dst: Failed},
		},
		fsm.Callbacks{
			OnStarting: func(ctx context.Context, e *fsm.Event) {
				service.MarkStarting()

				if !loader.Has(serviceName) {
					loader.Start(serviceName, fmt.Sprintf("Starting %s…", serviceName))
				}
			},
			OnStopping: func(ctx context.Context, e *fsm.Event) {
				loader.Start(serviceName, fmt.Sprintf("Stopping %s…", serviceName))
				service.MarkStopping()
			},
			OnRestarting: func(ctx context.Context, e *fsm.Event) {
				loader.Start(serviceName, fmt.Sprintf("Restarting %s…", serviceName))
			},
			OnRunning: func(ctx context.Context, e *fsm.Event) {
				service.MarkRunning()
			},
			OnStopped: func(ctx context.Context, e *fsm.Event) {
				service.MarkStopped()
			},
			OnFailed: func(ctx context.Context, e *fsm.Event) {
				service.MarkFailed()
			},
		},
	)
}
