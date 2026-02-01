package runner

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"fuku/internal/app/bus"
	"fuku/internal/app/discovery"
	"fuku/internal/app/errors"
	"fuku/internal/app/logs"
	"fuku/internal/app/registry"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Runner defines the interface for service orchestration
type Runner interface {
	Run(ctx context.Context, profile string) error
}

// runner implements the Runner interface
type runner struct {
	cfg       *config.Config
	discovery discovery.Discovery
	registry  registry.Registry
	service   Service
	pool      WorkerPool
	bus       bus.Bus
	server    logs.Server
	log       logger.Logger
}

// NewRunner creates a new runner instance
func NewRunner(
	cfg *config.Config,
	disc discovery.Discovery,
	reg registry.Registry,
	service Service,
	pool WorkerPool,
	bus bus.Bus,
	server logs.Server,
	log logger.Logger,
) Runner {
	return &runner{
		cfg:       cfg,
		discovery: disc,
		registry:  reg,
		service:   service,
		pool:      pool,
		bus:       bus,
		server:    server,
		log:       log.WithComponent("RUNNER"),
	}
}

// Run executes the specified profile
func (r *runner) Run(ctx context.Context, profile string) error {
	if err := r.server.Start(ctx, profile); err != nil {
		r.log.Warn().Err(err).Msg("Failed to start logs server, continuing without it")
	} else {
		defer r.server.Stop()
	}

	r.bus.Publish(bus.Message{
		Type:     bus.EventPhaseChanged,
		Data:     bus.PhaseChanged{Phase: bus.PhaseStartup},
		Critical: true,
	})

	tiers, err := r.discovery.Resolve(profile)
	if err != nil {
		return fmt.Errorf("failed to resolve profile: %w", err)
	}

	tierData := make([]bus.Tier, len(tiers))
	for i, tier := range tiers {
		tierData[i] = bus.Tier{Name: tier.Name, Services: tier.Services}
	}

	r.bus.Publish(bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: profile,
			Tiers:   tierData,
		},
		Critical: true,
	})

	services := r.collectServices(tiers)
	if len(services) == 0 {
		r.log.Warn().Msgf("No services found for profile '%s'. Nothing to run.", profile)
		r.bus.Publish(bus.Message{
			Type:     bus.EventPhaseChanged,
			Data:     bus.PhaseChanged{Phase: bus.PhaseStopped},
			Critical: true,
		})

		return nil
	}

	r.log.Info().Msgf("Starting services in profile '%s': %v", profile, services)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	msgChan := r.bus.Subscribe(ctx)
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	if err := r.runStartupPhase(ctx, cancel, tiers, sigChan, msgChan); err != nil {
		r.bus.Publish(bus.Message{
			Type:     bus.EventPhaseChanged,
			Data:     bus.PhaseChanged{Phase: bus.PhaseStopped},
			Critical: true,
		})

		return err
	}

	r.bus.Publish(bus.Message{
		Type:     bus.EventPhaseChanged,
		Data:     bus.PhaseChanged{Phase: bus.PhaseRunning},
		Critical: true,
	})
	r.runServicePhase(ctx, cancel, sigChan, msgChan)
	r.bus.Publish(bus.Message{
		Type:     bus.EventPhaseChanged,
		Data:     bus.PhaseChanged{Phase: bus.PhaseStopping},
		Critical: true,
	})

	r.shutdown()
	r.log.Info().Msg("All services stopped")

	r.bus.Publish(bus.Message{
		Type:     bus.EventPhaseChanged,
		Data:     bus.PhaseChanged{Phase: bus.PhaseStopped},
		Critical: true,
	})

	return nil
}

// runStartupPhase handles the service startup phase
func (r *runner) runStartupPhase(ctx context.Context, cancel context.CancelFunc, tiers []discovery.Tier, sigChan chan os.Signal, msgChan <-chan bus.Message) error {
	startupDone := make(chan struct{}, 1)

	go func() {
		r.startAllTiers(ctx, tiers)

		startupDone <- struct{}{}
	}()

	for {
		select {
		case <-startupDone:
			r.log.Info().Msg("Startup phase complete, waiting for signals...")

			return nil
		case sig := <-sigChan:
			r.log.Info().Msgf("Received signal %s during startup, shutting down services...", sig)
			r.bus.Publish(bus.Message{
				Type:     bus.EventSignal,
				Data:     bus.Signal{Name: sig.String()},
				Critical: true,
			})
			cancel()
			<-startupDone
			r.shutdown()

			return fmt.Errorf("%w: signal %s", errors.ErrStartupInterrupted, sig)
		case <-ctx.Done():
			r.log.Info().Msg("Context cancelled during startup, shutting down services...")
			cancel()
			<-startupDone
			r.shutdown()

			return ctx.Err()
		case msg, ok := <-msgChan:
			if !ok {
				r.log.Info().Msg("Message channel closed during startup, shutting down services...")
				cancel()
				<-startupDone
				r.shutdown()

				return errors.ErrCommandChannelClosed
			}

			if msg.Type == bus.CommandStopAll {
				r.log.Info().Msg("Received StopAll command during startup, shutting down services...")
				cancel()
				<-startupDone
				r.shutdown()

				return fmt.Errorf("%w: StopAll command", errors.ErrStartupInterrupted)
			}

			r.log.Debug().Msgf("Handling message during startup: %v", msg.Type)
			r.handleCommand(ctx, msg)
		}
	}
}

// runServicePhase runs the main event loop
func (r *runner) runServicePhase(ctx context.Context, cancel context.CancelFunc, sigChan chan os.Signal, msgChan <-chan bus.Message) {
	for {
		select {
		case sig := <-sigChan:
			r.log.Info().Msgf("Received signal %s, shutting down services...", sig)
			r.bus.Publish(bus.Message{
				Type:     bus.EventSignal,
				Data:     bus.Signal{Name: sig.String()},
				Critical: true,
			})
			cancel()

			return
		case <-ctx.Done():
			r.log.Info().Msg("Context cancelled, shutting down services...")

			return
		case msg, ok := <-msgChan:
			if !ok {
				return
			}

			if r.handleMessage(ctx, msg) {
				cancel()

				return
			}
		}
	}
}

// handleMessage processes a message and returns true if shutdown requested
func (r *runner) handleMessage(ctx context.Context, msg bus.Message) bool {
	switch msg.Type {
	case bus.EventWatchTriggered:
		if data, ok := msg.Data.(bus.WatchTriggered); ok {
			go func(service string, files []string) {
				if err := r.pool.Acquire(ctx); err != nil {
					r.log.Warn().Err(err).Msgf("Failed to acquire worker for watch restart of '%s'", service)

					return
				}
				defer r.pool.Release()

				r.log.Info().Msgf("File change detected for service '%s': %v", service, files)
				r.service.Restart(ctx, service)
			}(data.Service, data.ChangedFiles)
		}

		return false
	default:
		return r.handleCommand(ctx, msg)
	}
}

// handleCommand processes a command and returns true if shutdown requested
func (r *runner) handleCommand(ctx context.Context, msg bus.Message) bool {
	if msg.Type == bus.CommandStopAll {
		r.log.Info().Msg("Received StopAll command, shutting down all services...")

		return true
	}

	data, ok := msg.Data.(bus.Payload)
	if !ok {
		return false
	}

	switch msg.Type {
	case bus.CommandStopService:
		r.service.Stop(data.Name)
	case bus.CommandRestartService:
		go r.service.Restart(ctx, data.Name)
	}

	return false
}

// startAllTiers starts services tier by tier
func (r *runner) startAllTiers(ctx context.Context, tiers []discovery.Tier) {
	for tierIdx, tier := range tiers {
		if len(tier.Services) == 0 {
			continue
		}

		r.log.Info().Msgf("Starting tier '%s' (%d/%d) with services: %v", tier.Name, tierIdx+1, len(tiers), tier.Services)
		r.bus.Publish(bus.Message{
			Type:     bus.EventTierStarting,
			Data:     bus.TierStarting{Name: tier.Name, Index: tierIdx + 1, Total: len(tiers)},
			Critical: true,
		})

		failed := r.startTier(ctx, tier.Name, tier.Services)

		if len(failed) > 0 {
			r.log.Warn().Msgf("Tier '%s' partially failed: %d/%d services failed: %v", tier.Name, len(failed), len(tier.Services), failed)
		} else {
			r.log.Info().Msgf("Tier '%s' started successfully, all services ready", tier.Name)
			r.bus.Publish(bus.Message{
				Type:     bus.EventTierReady,
				Data:     bus.Payload{Name: tier.Name},
				Critical: true,
			})
		}
	}
}

// startTier starts all services in a tier concurrently
func (r *runner) startTier(ctx context.Context, tier string, services []string) []string {
	failedChan := make(chan string, len(services))

	var wg sync.WaitGroup

	for _, name := range services {
		wg.Add(1)

		go func(name string) {
			defer wg.Done()

			if err := r.pool.Acquire(ctx); err != nil {
				r.log.Error().Err(err).Msgf("Failed to acquire worker for service '%s'", name)
				r.bus.Publish(bus.Message{
					Type:     bus.EventServiceFailed,
					Data:     bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: name, Tier: tier}, Error: fmt.Errorf("%w: %w", errors.ErrFailedToAcquireWorker, err)},
					Critical: true,
				})

				failedChan <- name

				return
			}
			defer r.pool.Release()

			if err := r.service.Start(ctx, name, tier); err != nil {
				failedChan <- name
			}
		}(name)
	}

	wg.Wait()
	close(failedChan)

	failed := make([]string, 0, len(services))
	for name := range failedChan {
		failed = append(failed, name)
	}

	return failed
}

// shutdown stops all services in reverse order
func (r *runner) shutdown() {
	processes := r.registry.SnapshotReverse()

	for _, proc := range processes {
		r.registry.Detach(proc.Name())
	}

	for _, proc := range processes {
		r.service.Stop(proc.Name())
	}

	r.registry.Wait()
}

func (r *runner) collectServices(tiers []discovery.Tier) []string {
	var services []string
	for _, tier := range tiers {
		services = append(services, tier.Services...)
	}

	return services
}
