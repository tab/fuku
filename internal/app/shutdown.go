package app

import (
	"os"
	"sync"
)

// Shutdown records what initiated the application shutdown
type Shutdown struct {
	mu     sync.Mutex
	signal os.Signal
	self   bool
}

// NewShutdown creates a new shutdown record
func NewShutdown() *Shutdown {
	return &Shutdown{}
}

// Self marks the shutdown as initiated by the application itself rather than by an OS signal
func (s *Shutdown) Self() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.self = true
}

// Observe records the OS signal that initiated the shutdown (ignored once the application claimed the shutdown as its own)
func (s *Shutdown) Observe(sig os.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.self {
		return
	}

	s.signal = sig
}

// Signal returns the OS signal that initiated the shutdown (nil when the application shut itself down)
func (s *Shutdown) Signal() os.Signal {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.signal
}
