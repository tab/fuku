package services

import (
	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Admission is an accepted action with the status it leads to
type Admission struct {
	Service model.Service
	Action  contracts.Action
	Status  model.Status
}

// Control validates and admits service actions for every frontend
type Control struct {
	guard     *Guard
	publisher contracts.Publisher
}

// NewControl creates the shared service control
func NewControl(guard *Guard, publisher contracts.Publisher) *Control {
	return &Control{guard: guard, publisher: publisher}
}

// Start admits a start of a stopped or failed service
func (c *Control) Start(id string) (Admission, error) {
	return c.admit(id, contracts.ActionStart)
}

// Stop admits a stop of a running service
func (c *Control) Stop(id string) (Admission, error) {
	return c.admit(id, contracts.ActionStop)
}

// Restart admits a restart of a running, stopped or failed service
func (c *Control) Restart(id string) (Admission, error) {
	return c.admit(id, contracts.ActionRestart)
}

// Toggle admits a stop of a service with a live child and a start of one without
func (c *Control) Toggle(id string) (Admission, error) {
	if c.guard.live(id) {
		return c.Stop(id)
	}

	return c.Start(id)
}

// StopAll closes admission and requests the shutdown of every service (ErrNotAccepting before a run, nil once stopping)
func (c *Control) StopAll() error {
	switch c.guard.halt() {
	case model.PhaseStopping, model.PhaseStopped:
		return nil
	case model.PhaseStartup, model.PhaseRunning:
		return c.publisher.Publish(contracts.Message{Type: contracts.CommandStopAll})
	default:
		return contracts.ErrNotAccepting
	}
}

// admit reserves the service and publishes the command, releasing the reservation when the publish is rejected
func (c *Control) admit(id string, action contracts.Action) (Admission, error) {
	admission, err := c.guard.admit(id, action)
	if err != nil {
		return Admission{}, err
	}

	if err := c.publisher.Publish(contracts.Message{Type: action.Command(), Data: admission.Service}); err != nil {
		c.guard.release(id)

		return Admission{}, err
	}

	return admission, nil
}
