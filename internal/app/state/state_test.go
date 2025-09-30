package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/app/runner"
)

func Test_NewManager(t *testing.T) {
	m := NewManager()
	assert.NotNil(t, m)

	instance, ok := m.(*manager)
	assert.True(t, ok)
	assert.Equal(t, runner.PhaseDiscovery, instance.phase)
	assert.NotNil(t, instance.services)
	assert.NotNil(t, instance.serviceOrder)
	assert.Equal(t, 0, len(instance.services))
	assert.Equal(t, 0, len(instance.serviceOrder))
	assert.Equal(t, 0, instance.totalServices)
}

func Test_GetPhase_SetPhase(t *testing.T) {
	m := NewManager()

	assert.Equal(t, runner.PhaseDiscovery, m.GetPhase())

	m.SetPhase(runner.PhaseExecution)
	assert.Equal(t, runner.PhaseExecution, m.GetPhase())

	m.SetPhase(runner.PhaseRunning)
	assert.Equal(t, runner.PhaseRunning, m.GetPhase())

	m.SetPhase(runner.PhaseShutdown)
	assert.Equal(t, runner.PhaseShutdown, m.GetPhase())
}

func Test_AddService(t *testing.T) {
	m := NewManager()

	svc1 := &ServiceStatus{Name: "service1", State: Starting, PID: 123}
	m.AddService(svc1)

	svc2 := &ServiceStatus{Name: "service2", State: Running, PID: 456}
	m.AddService(svc2)

	services := m.GetServices()
	assert.Len(t, services, 2)
	assert.Equal(t, svc1, services["service1"])
	assert.Equal(t, svc2, services["service2"])
}

func Test_AddService_Update(t *testing.T) {
	m := NewManager()

	svc1 := &ServiceStatus{Name: "service1", State: Starting, PID: 123}
	m.AddService(svc1)

	order := m.GetServiceOrder()
	assert.Len(t, order, 1)

	// Update the same service
	svc1Updated := &ServiceStatus{Name: "service1", State: Running, PID: 123}
	m.AddService(svc1Updated)

	// Order should still be 1 (not duplicated)
	order = m.GetServiceOrder()
	assert.Len(t, order, 1)
	assert.Equal(t, "service1", order[0])

	// Service should be updated
	retrieved, exists := m.GetService("service1")
	assert.True(t, exists)
	assert.Equal(t, Running, retrieved.State)
}

func Test_GetService(t *testing.T) {
	m := NewManager()

	svc := &ServiceStatus{Name: "service1", State: Running, PID: 123}
	m.AddService(svc)

	retrieved, exists := m.GetService("service1")
	assert.True(t, exists)
	assert.Equal(t, svc, retrieved)

	_, exists = m.GetService("nonexistent")
	assert.False(t, exists)
}

func Test_GetServices(t *testing.T) {
	m := NewManager()

	svc1 := &ServiceStatus{Name: "service1", State: Running, PID: 123}
	svc2 := &ServiceStatus{Name: "service2", State: Starting, PID: 456}
	m.AddService(svc1)
	m.AddService(svc2)

	services := m.GetServices()
	assert.Len(t, services, 2)
	assert.Equal(t, svc1, services["service1"])
	assert.Equal(t, svc2, services["service2"])
}

func Test_GetServiceOrder(t *testing.T) {
	m := NewManager()

	svc1 := &ServiceStatus{Name: "service1"}
	svc2 := &ServiceStatus{Name: "service2"}
	svc3 := &ServiceStatus{Name: "service3"}

	m.AddService(svc1)
	m.AddService(svc2)
	m.AddService(svc3)

	order := m.GetServiceOrder()
	assert.Equal(t, []string{"service1", "service2", "service3"}, order)
}

func Test_GetTotalServices_SetTotalServices(t *testing.T) {
	m := NewManager()

	assert.Equal(t, 0, m.GetTotalServices())

	m.SetTotalServices(5)
	assert.Equal(t, 5, m.GetTotalServices())

	m.SetTotalServices(10)
	assert.Equal(t, 10, m.GetTotalServices())
}

func Test_GetServiceCounts(t *testing.T) {
	m := NewManager()
	m.SetTotalServices(5)

	m.AddService(&ServiceStatus{Name: "svc1", State: Running})
	m.AddService(&ServiceStatus{Name: "svc2", State: Running})
	m.AddService(&ServiceStatus{Name: "svc3", State: Starting})
	m.AddService(&ServiceStatus{Name: "svc4", State: Stopped})
	m.AddService(&ServiceStatus{Name: "svc5", State: Failed})

	total, running, starting, stopped, failed := m.GetServiceCounts()

	assert.Equal(t, 5, total)
	assert.Equal(t, 2, running)
	assert.Equal(t, 1, starting)
	assert.Equal(t, 1, stopped)
	assert.Equal(t, 1, failed)
}

func Test_GetServiceCounts_Empty(t *testing.T) {
	m := NewManager()
	m.SetTotalServices(3)

	total, running, starting, stopped, failed := m.GetServiceCounts()

	assert.Equal(t, 3, total)
	assert.Equal(t, 0, running)
	assert.Equal(t, 0, starting)
	assert.Equal(t, 0, stopped)
	assert.Equal(t, 0, failed)
}

func Test_ServiceState_String(t *testing.T) {
	tests := []struct {
		name     string
		state    ServiceState
		expected string
	}{
		{"Starting", Starting, "Starting"},
		{"Running", Running, "Running"},
		{"Failed", Failed, "Failed"},
		{"Stopped", Stopped, "Stopped"},
		{"Stopping", Stopping, "Stopping"},
		{"Restarting", Restarting, "Restarting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.state.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_ServiceStatus_Fields(t *testing.T) {
	now := time.Now()
	svc := &ServiceStatus{
		Name:      "test-service",
		State:     Running,
		PID:       12345,
		StartTime: now,
		ExitCode:  0,
		CPUUsage:  25.5,
		MemUsage:  1024000,
	}

	assert.Equal(t, "test-service", svc.Name)
	assert.Equal(t, Running, svc.State)
	assert.Equal(t, 12345, svc.PID)
	assert.Equal(t, now, svc.StartTime)
	assert.Equal(t, 0, svc.ExitCode)
	assert.Equal(t, 25.5, svc.CPUUsage)
	assert.Equal(t, uint64(1024000), svc.MemUsage)
}
