package process

import (
	"time"

	"fuku/internal/contracts"
)

// Reporter receives a runtime failure the preflight cleanup cannot recover from
type Reporter interface {
	Fail(err error)
}

// publish sends a critical preflight event and reports a rejected publish as a runtime failure
func (p *Preflight) publish(msg contracts.Message) {
	if err := p.publisher.Publish(msg); err != nil {
		p.reporter.Fail(err)
	}
}

// publishStarted announces the scan over the named services
func (p *Preflight) publishStarted(services []string) {
	p.publish(contracts.Message{
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
	p.publish(contracts.Message{
		Type: contracts.EventPreflightComplete,
		Data: contracts.PreflightComplete{Killed: killed, Duration: duration},
	})
}
