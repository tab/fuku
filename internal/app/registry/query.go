package registry

import (
	"context"

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
