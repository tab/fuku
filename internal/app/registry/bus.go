package registry

import (
	"context"
	"fmt"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// projected lists the message types the read model is built from
var projected = []contracts.MessageType{
	contracts.EventProfileResolved,
	contracts.EventPhaseChanged,
	contracts.EventTierStarting,
	contracts.EventTierReady,
	contracts.EventServiceStarting,
	contracts.EventServiceReady,
	contracts.EventServiceFailed,
	contracts.EventServiceStopping,
	contracts.EventServiceStopped,
	contracts.EventServiceRestarting,
	contracts.EventWatchStarted,
	contracts.EventWatchStopped,
	contracts.EventAPIStarted,
	contracts.EventAPIStopped,
	contracts.EventServiceResourcesSampled,
}

// Subscribe registers the required subscription and projects events until ctx is cancelled
func (s *Store) Subscribe(ctx context.Context) error {
	sub, err := s.subscriber.Subscribe(ctx, contracts.SubscribeOptions{Name: "registry", Required: true, Types: projected})
	if err != nil {
		return fmt.Errorf("failed to subscribe the registry: %w", err)
	}

	s.loop = contracts.Run(ctx, sub, s.handle)

	return nil
}

// Drain returns once the queue is empty and no commit is in flight
func (s *Store) Drain(ctx context.Context) error {
	return s.loop.Drain(ctx)
}

// handle applies one message through update
func (s *Store) handle(msg contracts.Message) {
	s.update(func(snapshot *model.Snapshot) bool {
		return s.apply(snapshot, msg)
	})
}
