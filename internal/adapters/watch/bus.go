package watch

import (
	"context"
	"fmt"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// serviceTypes lists the lifecycle events that start and stop watching
var serviceTypes = []contracts.MessageType{contracts.EventServiceReady, contracts.EventServiceStopped}

// Subscribe registers the required subscription and manages file watching until ctx is cancelled
func (w *Watcher) Subscribe(ctx context.Context) error {
	sub, err := w.subscriber.Subscribe(ctx, contracts.SubscribeOptions{Name: "watcher", Required: true, Types: serviceTypes})
	if err != nil {
		return fmt.Errorf("failed to subscribe the watcher: %w", err)
	}

	w.loop = contracts.Run(ctx, sub, w.handleServiceEvent)

	return nil
}

// Drain returns once the queue is empty and no handler is in flight
func (w *Watcher) Drain(ctx context.Context) error {
	return w.loop.Drain(ctx)
}

// handleServiceEvent processes bus messages to start/stop watching
func (w *Watcher) handleServiceEvent(msg contracts.Message) {
	//nolint:exhaustive // only handling service ready/stopped events
	switch msg.Type {
	case contracts.EventServiceReady:
		if data, ok := msg.Data.(contracts.ServiceReady); ok {
			w.startWatching(data.Service)
		}
	case contracts.EventServiceStopped:
		if data, ok := msg.Data.(contracts.ServiceStopped); ok {
			w.stopWatching(data.Service.ID)
		}
	}
}

// publishWatching announces that a service is now watched or no longer watched
func (w *Watcher) publishWatching(msgType contracts.MessageType, data any) {
	//nolint:errcheck // a non-critical publish never fails
	w.publisher.Publish(contracts.Message{
		Type: msgType,
		Data: data,
	})
}

// publishTriggered announces a debounced batch of file changes for a service
func (w *Watcher) publishTriggered(svc model.Service, files []string) {
	w.mu.RLock()
	closed := w.closed
	w.mu.RUnlock()

	if closed {
		return
	}

	err := w.publisher.Publish(contracts.Message{
		Type: contracts.EventWatchTriggered,
		Data: contracts.WatchTriggered{Service: svc, ChangedFiles: files},
	})
	if err != nil {
		w.log.Warn(fmt.Sprintf("Failed to publish the file change for service '%s'", svc.Name), "error", err)
	}
}
