package environment

import (
	"context"
	"fmt"

	"fuku/internal/contracts"
)

// reloadTypes lists the lifecycle events that reload a service's .env entries
var reloadTypes = []contracts.MessageType{contracts.EventProfileResolved, contracts.EventServiceStarting}

// Subscribe registers the subscription and refreshes the cache on lifecycle events
func (s *Store) Subscribe(ctx context.Context) error {
	sub, err := s.subscriber.Subscribe(ctx, contracts.SubscribeOptions{Name: "environment", Types: reloadTypes})
	if err != nil {
		return fmt.Errorf("failed to subscribe the environment store: %w", err)
	}

	s.loop = contracts.Run(ctx, sub, s.handle)

	return nil
}

// Drain returns once the queue is empty and no reload is in flight
func (s *Store) Drain(ctx context.Context) error {
	if s.loop == nil {
		return nil
	}

	return s.loop.Drain(ctx)
}

func (s *Store) handle(msg contracts.Message) {
	//nolint:exhaustive // only lifecycle events trigger env reload
	switch msg.Type {
	case contracts.EventProfileResolved:
		s.handleProfileResolved(msg)
	case contracts.EventServiceStarting:
		s.handleServiceStarting(msg)
	}
}

func (s *Store) handleProfileResolved(msg contracts.Message) {
	data, ok := msg.Data.(contracts.ProfileResolved)
	if !ok {
		return
	}

	for _, tier := range data.Tiers {
		for _, svc := range tier.Services {
			s.reload(svc)
		}
	}
}

func (s *Store) handleServiceStarting(msg contracts.Message) {
	data, ok := msg.Data.(contracts.ServiceStarting)
	if !ok {
		return
	}

	s.reload(&data.Service)
}
