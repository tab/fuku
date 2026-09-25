package services

import (
	"context"
	"fmt"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Reporter receives a runtime failure the services cannot recover from
type Reporter interface {
	Fail(err error)
}

// commandTypes lists the messages the runtime consumes
var commandTypes = []contracts.MessageType{
	contracts.CommandStartService,
	contracts.CommandStopService,
	contracts.CommandRestartService,
	contracts.CommandStopAll,
	contracts.EventWatchTriggered,
}

// Subscribe registers the command subscription and handles commands on its own goroutine for the whole lifetime
func (r *Runtime) Subscribe(ctx context.Context) error {
	sub, err := r.subscriber.Subscribe(ctx, contracts.SubscribeOptions{Name: "services", Required: true, Types: commandTypes})
	if err != nil {
		return fmt.Errorf("failed to subscribe to service commands: %w", err)
	}

	r.loop = contracts.Run(ctx, sub, r.handle)

	return nil
}

// Drain returns once the command queue is empty and no handler is in flight
func (r *Runtime) Drain(ctx context.Context) error {
	return r.loop.Drain(ctx)
}

// publish sends a lifecycle event and reports a rejected publish as a runtime failure
func (r *Runtime) publish(msg contracts.Message) {
	if err := r.publisher.Publish(msg); err != nil {
		r.reporter.Fail(err)
	}
}

// handle runs one command or file change against the active run
func (r *Runtime) handle(msg contracts.Message) {
	if msg.Type == contracts.CommandStopAll {
		r.stopAll()

		return
	}

	switch data := msg.Data.(type) {
	case contracts.WatchTriggered:
		r.handleWatch(data)
	case model.Service:
		r.handleCommand(msg.Type, data)
	}
}

// handleCommand takes over the admission's token and runs the command's action on a worker (a stop needs none)
func (r *Runtime) handleCommand(cmd contracts.MessageType, svc model.Service) {
	//nolint:exhaustive // the subscription is filtered to the command types
	switch cmd {
	case contracts.CommandStopService:
		r.dispatch(svc.ID, func(context.Context) { r.stop(svc.ID) })
	case contracts.CommandStartService:
		r.dispatch(svc.ID, func(ctx context.Context) { r.withWorker(ctx, svc, r.start) })
	case contracts.CommandRestartService:
		r.dispatch(svc.ID, func(ctx context.Context) { r.withWorker(ctx, svc, r.restart) })
	}
}

// handleWatch restarts the changed service unless a reservation is already held for it
func (r *Runtime) handleWatch(data contracts.WatchTriggered) {
	svc := data.Service

	if !r.guard.reserve(svc.ID) {
		r.log.Debug(fmt.Sprintf("Service '%s' is busy, dropping the file change", svc.Name))

		return
	}

	r.dispatch(svc.ID, func(ctx context.Context) {
		r.withWorker(ctx, svc, func(ctx context.Context, svc model.Service) {
			r.log.Info(fmt.Sprintf("File change detected for service '%s': %v", svc.Name, data.ChangedFiles))
			r.restart(ctx, svc)
		})
	})
}

// dispatch runs an action on the run's work context and releases the service token after it (at once without a run)
func (r *Runtime) dispatch(id string, action func(ctx context.Context)) {
	r.mu.Lock()

	ctx := r.work
	if ctx == nil {
		r.mu.Unlock()
		r.guard.release(id)

		return
	}

	r.wg.Add(1)
	r.mu.Unlock()

	go func() {
		defer r.wg.Done()
		defer r.guard.release(id)

		action(ctx)
	}()
}

// withWorker acquires a worker slot before running a service action
func (r *Runtime) withWorker(ctx context.Context, svc model.Service, action func(context.Context, model.Service)) {
	if err := r.pool.Acquire(ctx); err != nil {
		r.log.Warn(fmt.Sprintf("Failed to acquire worker for service '%s'", svc.Name), "error", err)

		return
	}
	defer r.pool.Release()

	action(ctx, svc)
}
