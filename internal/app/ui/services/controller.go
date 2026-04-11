package services

import "fuku/internal/app/bus"

// Controller handles business logic for service orchestration
type Controller interface {
	Start(svc bus.Service)
	Stop(svc bus.Service)
	Restart(svc bus.Service)
	StopAll()
}

// controller implements the Controller interface
type controller struct {
	bus bus.Bus
}

// NewController creates a new controller with the given bus
func NewController(b bus.Bus) Controller {
	return &controller{
		bus: b,
	}
}

// Start requests a service start
func (c *controller) Start(svc bus.Service) {
	c.bus.Publish(bus.Message{
		Type:     bus.CommandStartService,
		Data:     svc,
		Critical: true,
	})
}

// Stop requests a service stop
func (c *controller) Stop(svc bus.Service) {
	c.bus.Publish(bus.Message{
		Type:     bus.CommandStopService,
		Data:     svc,
		Critical: true,
	})
}

// Restart requests a service restart
func (c *controller) Restart(svc bus.Service) {
	c.bus.Publish(bus.Message{
		Type:     bus.CommandRestartService,
		Data:     svc,
		Critical: true,
	})
}

// StopAll sends a command to stop all services
func (c *controller) StopAll() {
	c.bus.Publish(bus.Message{Type: bus.CommandStopAll, Critical: true})
}
