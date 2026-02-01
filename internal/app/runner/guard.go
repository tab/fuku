package runner

import (
	"sync"
)

// Guard prevents concurrent restarts of the same service during hot-reload
type Guard interface {
	Lock(name string) bool
	Unlock(name string)
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
func (g *guard) Lock(name string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.active[name] {
		return false
	}

	g.active[name] = true

	return true
}

// Unlock releases the restart lock for the service
func (g *guard) Unlock(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.active, name)
}
