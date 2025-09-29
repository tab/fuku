//go:generate mockgen -source=result.go -destination=result_mock.go -package=tracker
package tracker

import "sync"

// Result defines the interface for service result management
type Result interface {
	GetName() string
	GetStatus() Status
	SetStatus(status Status)
	GetError() error
	SetError(err error)
}

// Status represents the current state of a service
type Status int

const (
	StatusPending Status = iota
	StatusStarting
	StatusRunning
	StatusFailed
	StatusStopped
)

// ServiceResult holds the status and error information for a service
type ServiceResult struct {
	Name   string
	Status Status
	Error  error
	mu     sync.RWMutex
}

// NewResult creates a new service result with pending status
func NewResult(name string) Result {
	return &ServiceResult{
		Name:   name,
		Status: StatusPending,
	}
}

// GetName returns the service name
func (sr *ServiceResult) GetName() string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.Name
}

// GetStatus returns the current status
func (sr *ServiceResult) GetStatus() Status {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.Status
}

// SetStatus safely updates the service status
func (sr *ServiceResult) SetStatus(status Status) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.Status = status
}

// GetError returns the current error
func (sr *ServiceResult) GetError() error {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.Error
}

// SetError safely sets the error for the service
func (sr *ServiceResult) SetError(err error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.Error = err
}
