package services

import (
	"context"
	"fmt"
	"io"
	"time"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// start launches a stopped or failed service again, reaping a tracked child that exited
func (r *Runtime) start(ctx context.Context, svc model.Service) {
	if proc, tracked := r.tracker.Get(svc.ID); tracked {
		r.terminate(proc)
	}

	//nolint:errcheck // the failure is published by startWithRetry
	r.startWithRetry(ctx, svc)
}

// restart stops the service's child when it has one and launches the service once
func (r *Runtime) restart(ctx context.Context, svc model.Service) {
	tier := svc.Tier

	r.log.Info(fmt.Sprintf("Restarting service '%s'", svc.Name))
	r.publish(contracts.Message{
		Type: contracts.EventServiceRestarting,
		Data: contracts.ServiceRestarting{
			ServiceEvent: contracts.ServiceEvent{Service: svc, Tier: tier},
		},
	})

	if proc, running := r.tracker.Get(svc.ID); running {
		r.log.Info(fmt.Sprintf("Stopping service '%s' before restart", svc.Name))
		r.terminate(proc)
	}

	proc, err := r.attempt(ctx, svc, 1)
	if err != nil && ctx.Err() != nil {
		r.publishStopped(svc)

		return
	}

	if err != nil {
		r.log.Error(fmt.Sprintf("Failed to restart service '%s'", svc.Name), "error", err)
		r.publish(contracts.Message{
			Type: contracts.EventServiceFailed,
			Data: contracts.ServiceFailed{
				ServiceEvent: contracts.ServiceEvent{Service: svc, Tier: tier},
				Error:        err,
			},
		})

		return
	}

	r.watchForExit(proc)
}

// stop terminates the service's child as an intended stop and does nothing for a service without one
func (r *Runtime) stop(id string) {
	proc, exists := r.tracker.Get(id)
	if !exists {
		return
	}

	svc := proc.Service()
	tier := svc.Tier

	r.log.Info(fmt.Sprintf("Stopping service '%s'", svc.Name))
	r.publish(contracts.Message{
		Type: contracts.EventServiceStopping,
		Data: contracts.ServiceStopping{
			ServiceEvent: contracts.ServiceEvent{Service: svc, Tier: tier},
		},
	})

	r.terminate(proc)

	r.log.Info(fmt.Sprintf("Service '%s' stopped", svc.Name))
	r.publish(contracts.Message{
		Type: contracts.EventServiceStopped,
		Data: contracts.ServiceStopped{
			ServiceEvent: contracts.ServiceEvent{Service: svc, Tier: tier},
		},
	})
}

// publishStopped ends a service whose start or restart the end of the run cut short
func (r *Runtime) publishStopped(svc model.Service) {
	r.publish(contracts.Message{
		Type: contracts.EventServiceStopped,
		Data: contracts.ServiceStopped{
			ServiceEvent: contracts.ServiceEvent{Service: svc, Tier: svc.Tier},
		},
	})
}

// attempt runs one start attempt (port pre-check, launch, readiness) and terminates a child that never became ready
func (r *Runtime) attempt(ctx context.Context, svc model.Service, number int) (contracts.Process, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := r.probePort(svc.Name, svc.Readiness); err != nil {
		return nil, err
	}

	proc, err := r.launch(ctx, svc)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now()

	r.publish(contracts.Message{
		Type: contracts.EventServiceStarting,
		Data: contracts.ServiceStarting{
			ServiceEvent: contracts.ServiceEvent{Service: svc, Tier: svc.Tier},
			PID:          proc.PID(),
			Attempt:      number,
			StartedAt:    startedAt,
		},
	})

	if err := r.waitForReady(ctx, svc.Readiness, proc); err != nil {
		_ = proc.Terminate()
		r.tracker.Untrack(svc.ID, proc)

		return nil, err
	}

	r.publish(contracts.Message{
		Type: contracts.EventServiceReady,
		Data: contracts.ServiceReady{
			ServiceEvent: contracts.ServiceEvent{Service: svc, Tier: svc.Tier},
			PID:          proc.PID(),
			StartedAt:    startedAt,
			Duration:     time.Since(startedAt),
		},
	})

	return proc, nil
}

// launch checks cancellation and starts the child under one lock, so it is tracked before the run closes or not at all
func (r *Runtime) launch(ctx context.Context, svc model.Service) (contracts.Process, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return r.launcher.Start(svc)
}

// terminate stops a tracked child as an intended stop, so its exit is not reported
func (r *Runtime) terminate(proc contracts.Process) {
	id := proc.Service().ID

	r.tracker.Detach(id)

	_ = proc.Terminate()
	<-proc.Done()

	r.tracker.Untrack(id, proc)
}

// waitForReady runs the readiness check while keeping the process streams flowing (a log check reads them itself first)
func (r *Runtime) waitForReady(ctx context.Context, readiness *model.Readiness, proc contracts.Process) error {
	stdout := proc.Stdout()
	stderr := proc.Stderr()

	if readiness == nil {
		go drainPipe(stdout)
		go drainPipe(stderr)

		return nil
	}

	if readiness.Type != model.ReadinessLog {
		go drainPipe(stdout)
		go drainPipe(stderr)

		return r.check(ctx, *readiness, proc)
	}

	err := r.check(ctx, *readiness, proc)

	go drainPipe(stdout)
	go drainPipe(stderr)

	return err
}

// check runs the readiness check and names a failure
func (r *Runtime) check(ctx context.Context, readiness model.Readiness, proc contracts.Process) error {
	if err := r.readiness.Check(ctx, readiness, proc); err != nil {
		return fmt.Errorf("readiness check failed: %w", err)
	}

	return nil
}

// watchForExit reports an exit nobody asked for: a failure for a watched service, an unexpected stop otherwise
func (r *Runtime) watchForExit(proc contracts.Process) {
	go func() {
		<-proc.Done()

		svc := proc.Service()

		if !r.tracker.Untrack(svc.ID, proc) {
			return
		}

		tier := svc.Tier

		r.log.Info(fmt.Sprintf("Service '%s' exited unexpectedly", svc.Name))

		if svc.Watch != nil {
			r.publish(contracts.Message{
				Type: contracts.EventServiceFailed,
				Data: contracts.ServiceFailed{
					ServiceEvent: contracts.ServiceEvent{Service: svc, Tier: tier},
					Error:        contracts.ErrUnexpectedExit,
				},
			})

			return
		}

		r.publish(contracts.Message{
			Type: contracts.EventServiceStopped,
			Data: contracts.ServiceStopped{
				ServiceEvent: contracts.ServiceEvent{Service: svc, Tier: tier},
				Unexpected:   true,
			},
		})
	}()
}

// probePort fails the attempt before any launch when the service address is already in use
func (r *Runtime) probePort(name string, readiness *model.Readiness) error {
	if readiness == nil {
		return nil
	}

	port := r.readiness.ProbePort(*readiness)
	if !port.InUse {
		return nil
	}

	r.log.Warn(fmt.Sprintf("Service '%s' address %s is already in use", name, port.Address))

	return fmt.Errorf("%w: %s", contracts.ErrPortAlreadyInUse, port.Address)
}

func drainPipe(reader io.Reader) {
	//nolint:errcheck // intentionally draining pipe
	io.Copy(io.Discard, reader)
}
