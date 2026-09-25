package process

import (
	"time"

	"fuku/internal/contracts"
)

// publishStarted announces the scan over the named services
func (p *Preflight) publishStarted(services []string) {
	//nolint:errcheck // a non-critical publish never fails
	p.publisher.Publish(contracts.Message{
		Type: contracts.EventPreflightStarted,
		Data: contracts.PreflightStarted{Services: services},
	})
}

// publishKilled announces one process killed for a service
func (p *Preflight) publishKilled(service, name string, pid int) {
	//nolint:errcheck // a non-critical publish never fails
	p.publisher.Publish(contracts.Message{
		Type: contracts.EventPreflightKilled,
		Data: contracts.PreflightKilled{
			Service: service,
			Name:    name,
			PID:     pid,
		},
	})
}

// publishComplete announces the end of the scan with how many processes it killed
func (p *Preflight) publishComplete(killed int, duration time.Duration) {
	//nolint:errcheck // a non-critical publish never fails
	p.publisher.Publish(contracts.Message{
		Type: contracts.EventPreflightComplete,
		Data: contracts.PreflightComplete{Killed: killed, Duration: duration},
	})
}
