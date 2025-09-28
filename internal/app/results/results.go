package results

import (
	"sync"
)

// ServiceStatus represents the current state of a service
type ServiceStatus int

const (
	StatusPending ServiceStatus = iota
	StatusStarting
	StatusRunning
	StatusFailed
	StatusStopped
)

// ServiceResult holds the status and error information for a service
type ServiceResult struct {
	Name   string
	Status ServiceStatus
	Error  error
	mu     sync.RWMutex
}

// NewServiceResult creates a new service result with pending status
func NewServiceResult(name string) *ServiceResult {
	return &ServiceResult{
		Name:   name,
		Status: StatusPending,
	}
}

// UpdateStatus safely updates the service status
func (sr *ServiceResult) UpdateStatus(status ServiceStatus) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.Status = status
}

// SetError safely sets the error for the service
func (sr *ServiceResult) SetError(err error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.Error = err
}


// ResultsTracker manages and displays service results
type ResultsTracker struct {
	results map[string]*ServiceResult
	mu      sync.RWMutex
}

// NewResultsTracker creates a new results tracker
func NewResultsTracker() *ResultsTracker {
	return &ResultsTracker{
		results: make(map[string]*ServiceResult),
	}
}

// AddService adds a new service to track
func (rt *ResultsTracker) AddService(name string) *ServiceResult {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	result := NewServiceResult(name)
	rt.results[name] = result
	return result
}


