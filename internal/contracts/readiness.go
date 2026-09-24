package contracts

import (
	"time"

	"fuku/internal/model"
)

// EventReadinessComplete carries a ReadinessComplete
const EventReadinessComplete MessageType = "readiness_complete"

// ReadinessComplete indicates a readiness check has finished successfully
type ReadinessComplete struct {
	Service  model.Service
	Type     model.ReadinessType
	Duration time.Duration
}
