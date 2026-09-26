package tui

import "fuku/internal/model"

// transitional holds the statuses between two settled ones, which the view blinks and colours as starting
var transitional = map[model.Status]bool{
	model.StatusStarting:   true,
	model.StatusStopping:   true,
	model.StatusRestarting: true,
}

// updateBlinkAnimations updates blink state for services in transition states
func (m *Model) updateBlinkAnimations() bool {
	hasActiveBlinking := false

	for id, view := range m.state.views {
		switch {
		case transitional[m.snapshot.Services[id].Status]:
			if !view.Blink.IsActive() {
				view.Blink.Start()
			}

			view.Blink.Update()

			hasActiveBlinking = true
		default:
			if view.Blink.IsActive() {
				view.Blink.Stop()
			}
		}
	}

	return hasActiveBlinking
}
