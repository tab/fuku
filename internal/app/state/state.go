package state

//go:generate mockgen -source=state.go -destination=state_mock.go -package=state

import (
	"time"

	"fuku/internal/app/runner"
)

// ServiceState represents the current state of a service
type ServiceState int

const (
	Starting ServiceState = iota
	Initializing
	Ready
	Running
	Failed
	Stopped
	Stopping
	Restarting
)

// ServiceStatus contains runtime status information for a service
type ServiceStatus struct {
	Name      string
	State     ServiceState
	PID       int
	StartTime time.Time
	ExitCode  int
	Err       error
	CPUUsage  float64
	MemUsage  uint64
}

// Manager defines the interface for service state management
type Manager interface {
	GetPhase() runner.Phase
	SetPhase(phase runner.Phase)
	GetServices() map[string]*ServiceStatus
	GetService(name string) (*ServiceStatus, bool)
	AddService(svc *ServiceStatus)
	GetServiceOrder() []string
	GetTotalServices() int
	SetTotalServices(total int)
	GetServiceCounts() (total, running, starting, stopped, failed int)
}

// manager manages service lifecycle and state
type manager struct {
	phase         runner.Phase
	services      map[string]*ServiceStatus
	serviceOrder  []string
	totalServices int
}

// NewManager creates a new state manager
func NewManager() Manager {
	return &manager{
		phase:        runner.PhaseDiscovery,
		services:     make(map[string]*ServiceStatus),
		serviceOrder: []string{},
	}
}

// GetPhase returns the current phase
func (m *manager) GetPhase() runner.Phase {
	return m.phase
}

// SetPhase updates the current phase
func (m *manager) SetPhase(phase runner.Phase) {
	m.phase = phase
}

// GetServices returns all services
func (m *manager) GetServices() map[string]*ServiceStatus {
	return m.services
}

// GetService returns a specific service by name
func (m *manager) GetService(name string) (*ServiceStatus, bool) {
	svc, exists := m.services[name]
	return svc, exists
}

// AddService adds or updates a service
func (m *manager) AddService(svc *ServiceStatus) {
	if _, exists := m.services[svc.Name]; !exists {
		m.serviceOrder = append(m.serviceOrder, svc.Name)
	}
	m.services[svc.Name] = svc
}

// GetServiceOrder returns the ordered list of service names
func (m *manager) GetServiceOrder() []string {
	return m.serviceOrder
}

// GetTotalServices returns the total number of services discovered
func (m *manager) GetTotalServices() int {
	return m.totalServices
}

// SetTotalServices sets the total number of services discovered
func (m *manager) SetTotalServices(total int) {
	m.totalServices = total
}

// GetServiceCounts returns counts of services by state
func (m *manager) GetServiceCounts() (total, running, starting, stopped, failed int) {
	total = m.totalServices
	for _, svc := range m.services {
		switch svc.State {
		case Ready, Running, Stopping:
			running++
		case Starting, Initializing, Restarting:
			starting++
		case Stopped:
			stopped++
		case Failed:
			failed++
		}
	}
	return
}

// String returns a string representation of the service state
func (s ServiceState) String() string {
	switch s {
	case Starting:
		return "Starting"
	case Initializing:
		return "Initializing"
	case Ready:
		return "Ready"
	case Running:
		return "Running"
	case Failed:
		return "Failed"
	case Stopped:
		return "Stopped"
	case Stopping:
		return "Stopping"
	case Restarting:
		return "Restarting"
	default:
		return "Unknown"
	}
}
