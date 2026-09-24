package readiness

import (
	"time"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// publishComplete announces a passed readiness check and how long it took
func (c *Checker) publishComplete(svc model.Service, readinessType model.ReadinessType, duration time.Duration) {
	//nolint:errcheck // a non-critical publish never fails
	c.publisher.Publish(contracts.Message{
		Type: contracts.EventReadinessComplete,
		Data: contracts.ReadinessComplete{
			Service:  svc,
			Type:     readinessType,
			Duration: duration,
		},
	})
}
