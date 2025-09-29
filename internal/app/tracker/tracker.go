//go:generate mockgen -source=tracker.go -destination=tracker_mock.go -package=tracker
package tracker

import "sync"

// Tracker defines the interface for managing service results
type Tracker interface {
	Add(name string) Result
}

// tracker manages and displays service results
type tracker struct {
	results map[string]Result
	mu      sync.RWMutex
}

// NewTracker creates a new results tracker
func NewTracker() Tracker {
	return &tracker{
		results: make(map[string]Result),
	}
}

// Add adds a new service to track
func (rt *tracker) Add(name string) Result {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	r := NewResult(name)
	rt.results[name] = r

	return r
}
