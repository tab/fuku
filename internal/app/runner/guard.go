package runner

import (
	"sync"
)

// Guard prevents concurrent restarts of the same service during hot-reload
type Guard interface {
	Lock(id string) bool
	Unlock(id string)
}

// guard implements Guard interface
type guard struct {
	mu     sync.Mutex
	active map[string]bool
}

// NewGuard creates a new Guard instance
func NewGuard() Guard {
	return &guard{
		active: make(map[string]bool),
	}
}

// Lock attempts to acquire a restart lock for the service, returns true if acquired
func (g *guard) Lock(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.active[id] {
		return false
	}

	g.active[id] = true

	return true
}

// Unlock releases the restart lock for the service
func (g *guard) Unlock(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.active, id)
}
