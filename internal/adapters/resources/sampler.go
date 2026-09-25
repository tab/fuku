package resources

import (
	"context"
	"os"
	"time"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Sampling cadences: the fuku process feeds telemetry, the service processes feed the registry (bounded per read)
const (
	processInterval = 5 * time.Minute
	serviceInterval = 2 * time.Second
	serviceTimeout  = 200 * time.Millisecond
)

// Monitor reads the resource usage of a process
type Monitor interface {
	GetStats(ctx context.Context, pid int) (Stats, error)
}

// Registry exposes the runtime read model the service PIDs are read from
type Registry interface {
	Read(fn func(*model.Snapshot))
}

// Sampler periodically samples the fuku process and the service processes and publishes the readings
type Sampler struct {
	options   Options
	publisher contracts.Publisher
	monitor   Monitor
	registry  Registry
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewSampler creates a new resource sampler
func NewSampler(options Options, publisher contracts.Publisher, monitor Monitor, registry Registry) *Sampler {
	return &Sampler{
		options:   options,
		publisher: publisher,
		monitor:   monitor,
		registry:  registry,
		done:      make(chan struct{}),
	}
}

// Start samples on its own goroutine until Stop
func (s *Sampler) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)

	go func() {
		defer close(s.done)

		s.run(ctx)
	}()

	return nil
}

// Stop ends the sampling and waits for the last reading to finish
func (s *Sampler) Stop(ctx context.Context) error {
	s.cancel()

	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run samples the service processes every tick, and the fuku process when enabled, until ctx is cancelled
func (s *Sampler) run(ctx context.Context) {
	services := time.NewTicker(serviceInterval)
	defer services.Stop()

	var process <-chan time.Time

	if s.options.Enabled {
		s.prime(ctx)

		ticker := time.NewTicker(processInterval)
		defer ticker.Stop()

		process = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-process:
			s.sampleProcess(ctx)
		case <-services.C:
			s.sampleServices(ctx)
		}
	}
}

// prime warms up the CPU accounting so the first tick has a valid delta
func (s *Sampler) prime(ctx context.Context) {
	//nolint:errcheck // priming call; result is intentionally discarded
	s.monitor.GetStats(ctx, os.Getpid())
}

func (s *Sampler) sampleProcess(ctx context.Context) {
	stats, err := s.monitor.GetStats(ctx, os.Getpid())
	if err != nil {
		return
	}

	if stats.CPU <= 0 && stats.MEM <= 0 {
		return
	}

	s.publishProcess(stats)
}

// sampleServices reads every service with a live PID and publishes the readings as one batch
func (s *Sampler) sampleServices(ctx context.Context) {
	var samples []contracts.ServiceResourceSample

	for _, target := range s.targets() {
		readCtx, cancel := context.WithTimeout(ctx, serviceTimeout)
		stats, err := s.monitor.GetStats(readCtx, target.PID)

		cancel()

		if err != nil {
			continue
		}

		target.CPU = stats.CPU
		target.Memory = stats.RawMEM
		samples = append(samples, target)
	}

	if len(samples) == 0 {
		return
	}

	s.publishServices(samples)
}

// targets collects the services with a live PID in tier order under the read lock, so no reading holds it
func (s *Sampler) targets() []contracts.ServiceResourceSample {
	var targets []contracts.ServiceResourceSample

	s.registry.Read(func(snapshot *model.Snapshot) {
		for _, tier := range snapshot.Tiers {
			for _, svc := range tier.Services {
				if svc.Process.PID == 0 {
					continue
				}

				targets = append(targets, contracts.ServiceResourceSample{ID: svc.ID, PID: svc.Process.PID})
			}
		}
	})

	return targets
}
