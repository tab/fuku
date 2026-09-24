package tui

import "fuku/internal/model"

// updateBlinkAnimations updates blink state for services in transition states
func (m *Model) updateBlinkAnimations() bool {
	hasActiveBlinking := false

	for id, view := range m.state.views {
		switch m.snapshot.Services[id].Status {
		case model.StatusStarting, model.StatusStopping, model.StatusRestarting:
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
