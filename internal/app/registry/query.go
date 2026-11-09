package registry

import (
	"context"
	"time"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// WaitResolved blocks until the store has received the first ProfileResolved event or the context is cancelled
func (s *Store) WaitResolved(ctx context.Context) {
	select {
	case <-s.resolved:
	case <-ctx.Done():
	}
}

// Read runs fn under the read lock (fn must not keep the snapshot or anything it points to)
func (s *Store) Read(fn func(*model.Snapshot)) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fn(s.snapshot)
}

// WaitSettled blocks until the service is running and watched if it watches, failed or stopped since the time given
func (s *Store) WaitSettled(ctx context.Context, id string, since time.Time) error {
	for {
		s.mu.RLock()
		svc, known := s.snapshot.Services[id]
		settled := known && svc.SettledSince(since)
		changed := s.changed
		s.mu.RUnlock()

		if !known {
			return contracts.ErrServiceNotFound
		}

		if settled {
			return nil
		}

		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
