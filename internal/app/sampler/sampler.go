package sampler

import (
	"context"
	"os"
	"time"

	"fuku/internal/app/bus"
	"fuku/internal/app/monitor"
)

const sampleInterval = 5 * time.Minute

// Sampler periodically samples fuku process resource usage and publishes events
type Sampler interface {
	Run(ctx context.Context)
}

type sampler struct {
	bus     bus.Bus
	monitor monitor.Monitor
}

// NewSampler creates a new resource sampler
func NewSampler(bus bus.Bus, monitor monitor.Monitor) Sampler {
	return &sampler{
		bus:     bus,
		monitor: monitor,
	}
}

// Run samples CPU and memory at a fixed interval until context is cancelled
func (s *sampler) Run(ctx context.Context) {
	s.prime(ctx)

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

func (s *sampler) prime(ctx context.Context) {
	// Warm up CPU accounting so the first tick produces a valid delta
	//nolint:errcheck // priming call; result is intentionally discarded
	s.monitor.GetStats(ctx, os.Getpid())
}

func (s *sampler) sample(ctx context.Context) {
	stats, err := s.monitor.GetStats(ctx, os.Getpid())
	if err != nil {
		return
	}

	if stats.CPU <= 0 && stats.MEM <= 0 {
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
