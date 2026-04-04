package services

import (
	"fuku/internal/app/bus"
	"fuku/internal/app/registry"
)

// Controller handles business logic for service orchestration
type Controller interface {
	Start(name string) bool
	Stop(name string) bool
	Restart(name string) bool
	StopAll()
}

// controller implements the Controller interface
type controller struct {
	bus   bus.Bus
	store registry.Store
}

// NewController creates a new controller with the given bus and store
func NewController(b bus.Bus, store registry.Store) Controller {
	return &controller{
		bus:   b,
		store: store,
	}
}

// Start requests a service start if it's currently stopped or failed
func (c *controller) Start(name string) bool {
	svc, found := c.store.Service(name)
	if !found {
		return false
	}

	if !svc.Status.IsStartable() {
		return false
	}

	c.bus.Publish(bus.Message{
		Type:     bus.CommandStartService,
		Data:     bus.Payload{Name: name},
		Critical: true,
	})

	return true
}

// Stop requests a service stop if it's currently running
func (c *controller) Stop(name string) bool {
	svc, found := c.store.Service(name)
	if !found {
		return false
	}

	if !svc.Status.IsStoppable() {
		return false
	}

	c.bus.Publish(bus.Message{
		Type:     bus.CommandStopService,
		Data:     bus.Payload{Name: name},
		Critical: true,
	})

	return true
}

// Restart requests a service restart if it's running, failed, or stopped
func (c *controller) Restart(name string) bool {
	svc, found := c.store.Service(name)
	if !found {
		return false
	}

	if !svc.Status.IsRestartable() {
		return false
	}

	c.bus.Publish(bus.Message{
		Type:     bus.CommandRestartService,
		Data:     bus.Payload{Name: name},
		Critical: true,
	})

	return true
}

// StopAll sends a command to stop all services
func (c *controller) StopAll() {
	c.bus.Publish(bus.Message{Type: bus.CommandStopAll, Critical: true})
}
