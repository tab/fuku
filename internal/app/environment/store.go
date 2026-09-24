package environment

import (
	"sync"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Reader reads the entries of one .env file inside a service directory
type Reader interface {
	Read(dir, name string) ([]model.Env, error)
}

// Store caches the merged .env entries of every service for the UI's env tab (they never reach the service process)
type Store struct {
	subscriber contracts.Subscriber
	reader     Reader
	loop       *contracts.Loop

	mu    sync.RWMutex
	cache map[string][]model.Env
}

// NewStore creates a store that reads .env files through reader and caches merged entries per service ID
func NewStore(subscriber contracts.Subscriber, reader Reader) *Store {
	return &Store{
		subscriber: subscriber,
		reader:     reader,
		cache:      make(map[string][]model.Env),
	}
}

// Env returns the merged .env entries cached for the given service ID
func (s *Store) Env(id string) []model.Env {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := s.cache[id]
	if len(entries) == 0 {
		return nil
	}

	out := make([]model.Env, len(entries))
	copy(out, entries)

	return out
}
