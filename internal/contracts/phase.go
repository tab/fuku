package contracts

import (
	"time"

	"fuku/internal/model"
)

// EventPhaseChanged carries a PhaseChanged
const EventPhaseChanged MessageType = "phase_changed"

// PhaseChanged indicates an application phase transition
type PhaseChanged struct {
	Phase        model.Phase
	Duration     time.Duration
	ServiceCount int
}
