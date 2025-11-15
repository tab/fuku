package services

import (
	"context"
	"fmt"

	"github.com/looplab/fsm"

	"fuku/internal/app/runtime"
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

func newServiceFSM(serviceName string, model *Model) *fsm.FSM {
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
				if service := model.services[serviceName]; service != nil {
					service.MarkStarting()
				}

				if !model.loader.Has(serviceName) {
					model.loader.Start(serviceName, fmt.Sprintf("Starting %s…", serviceName))
				}
			},
			OnStopping: func(ctx context.Context, e *fsm.Event) {
				model.loader.Start(serviceName, fmt.Sprintf("Stopping %s…", serviceName))

				if service := model.services[serviceName]; service != nil {
					service.MarkStopping()
				}

				model.command.Publish(runtime.Command{
					Type: runtime.CommandStopService,
					Data: runtime.StopServiceData{Service: serviceName},
				})
			},
			OnRestarting: func(ctx context.Context, e *fsm.Event) {
				model.loader.Start(serviceName, fmt.Sprintf("Restarting %s…", serviceName))

				model.command.Publish(runtime.Command{
					Type: runtime.CommandRestartService,
					Data: runtime.RestartServiceData{Service: serviceName},
				})
			},
			OnRunning: func(ctx context.Context, e *fsm.Event) {
				if service := model.services[serviceName]; service != nil {
					service.MarkRunning()
				}
			},
			OnStopped: func(ctx context.Context, e *fsm.Event) {
				if service := model.services[serviceName]; service != nil {
					service.MarkStopped()
				}
			},
			OnFailed: func(ctx context.Context, e *fsm.Event) {
				if service := model.services[serviceName]; service != nil {
					service.MarkFailed()
				}
			},
		},
	)
}
