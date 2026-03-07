package sampler

import (
	"context"
	"os"
	"time"

	"fuku/internal/app/bus"
	"fuku/internal/app/monitor"
)

const sampleInterval = 30 * time.Second

// Sampler periodically samples fuku process resource usage and publishes events
type Sampler interface {
	Run(ctx context.Context)
}

type sampler struct {
	bus     bus.Bus
	monitor monitor.Monitor
}

// NewSampler creates a new resource sampler
func NewSampler(b bus.Bus, m monitor.Monitor) Sampler {
	return &sampler{
		bus:     b,
		monitor: m,
	}
}

// Run samples CPU and memory at a fixed interval until context is cancelled
func (s *sampler) Run(ctx context.Context) {
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sample(ctx)
		}
	}
}

func (s *sampler) sample(ctx context.Context) {
	stats, err := s.monitor.GetStats(ctx, os.Getpid())
	if err != nil {
		return
	}

	s.bus.Publish(bus.Message{
		Type: bus.EventResourceSample,
		Data: bus.ResourceSample{
			CPU: stats.CPU,
			MEM: stats.MEM,
		},
	})
}
