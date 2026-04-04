package runner

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"fuku/internal/app/api"
	"fuku/internal/app/bus"
	"fuku/internal/app/discovery"
	"fuku/internal/app/errors"
	"fuku/internal/app/preflight"
	"fuku/internal/app/registry"
	"fuku/internal/app/relay"
	"fuku/internal/app/worker"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Runner defines the interface for service orchestration
type Runner interface {
	Run(ctx context.Context, profile string) error
	Stop(ctx context.Context, profile string) error
}

// RunnerParams contains dependencies for creating a Runner
type RunnerParams struct {
	fx.In

	Config    *config.Config
	Discovery discovery.Discovery
	Preflight preflight.Preflight
	Registry  registry.Registry
	Service   Service
	Worker    worker.Pool
	Bus       bus.Bus
	Server    relay.Server
	API       api.Server
	Logger    logger.Logger
}

// runner implements the Runner interface
type runner struct {
	cfg        *config.Config
	discovery  discovery.Discovery
	preflight  preflight.Preflight
	registry   registry.Registry
	service    Service
	worker     worker.Pool
	bus        bus.Bus
	server     relay.Server
	api        api.Server
	log        logger.Logger
	apiStarted bool
}

// NewRunner creates a new runner instance
func NewRunner(p RunnerParams) Runner {
	return &runner{
		cfg:       p.Config,
		discovery: p.Discovery,
		registry:  p.Registry,
		preflight: p.Preflight,
		service:   p.Service,
		worker:    p.Worker,
		bus:       p.Bus,
		server:    p.Server,
		api:       p.API,
		log:       p.Logger.WithComponent("RUNNER"),
	}
}

// Run executes the specified profile
func (r *runner) Run(ctx context.Context, profile string) error {
	startupStart := time.Now()

	r.bus.Publish(bus.Message{
		Type:     bus.EventPhaseChanged,
		Data:     bus.PhaseChanged{Phase: bus.PhaseStartup},
		Critical: true,
	})

	discoveryStart := time.Now()

	tiers, err := r.discovery.Resolve(profile)
	if err != nil {
		return fmt.Errorf("failed to resolve profile: %w", err)
	}

	discoveryDuration := time.Since(discoveryStart)

	tierData := r.buildTiers(tiers)

	r.bus.Publish(bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile:  profile,
			Tiers:    tierData,
			Duration: discoveryDuration,
		},
		Critical: true,
	})

	services := r.collectServiceNames(tierData)

	if len(services) == 0 {
		r.log.Warn().Msgf("No services found for profile '%s'. Nothing to run.", profile)
		r.bus.Publish(bus.Message{
			Type:     bus.EventPhaseChanged,
			Data:     bus.PhaseChanged{Phase: bus.PhaseStopped},
			Critical: true,
		})

		return nil
	}

	dirs := r.resolveServiceDirs(services)
	if _, err := r.preflight.Cleanup(ctx, dirs); err != nil {
		r.log.Warn().Err(err).Msg("Preflight cleanup failed, continuing startup")
	}

	if err := relay.Cleanup(config.SocketDir); err != nil {
		r.log.Warn().Err(err).Msg("Socket cleanup failed, continuing startup")
	}

	if err := r.server.Start(ctx, profile, services); err != nil {
		r.log.Warn().Err(err).Msg("Failed to start logs server, continuing without it")
	} else {
		//nolint:errcheck // best-effort cleanup on shutdown
		defer r.server.Stop()
	}

	r.log.Info().Msgf("Starting services in profile '%s': %v", profile, services)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	msgChan := r.bus.Subscribe(ctx)
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	r.startAPI(ctx)
	defer r.stopAPI()

	if err := r.runStartupPhase(ctx, cancel, tierData, sigChan, msgChan); err != nil {
		r.bus.Publish(bus.Message{
			Type:     bus.EventPhaseChanged,
			Data:     bus.PhaseChanged{Phase: bus.PhaseStopped},
			Critical: true,
		})

		return err
	}

	r.bus.Publish(bus.Message{
		Type: bus.EventPhaseChanged,
		Data: bus.PhaseChanged{
			Phase:        bus.PhaseRunning,
			Duration:     time.Since(startupStart),
			ServiceCount: len(services),
		},
		Critical: true,
	})
	r.runServicePhase(ctx, cancel, sigChan, msgChan)

	r.bus.Publish(bus.Message{
		Type:     bus.EventPhaseChanged,
		Data:     bus.PhaseChanged{Phase: bus.PhaseStopping},
		Critical: true,
	})

	shutdownStart := time.Now()
	serviceCount := r.shutdown()
	r.log.Info().Msg("All services stopped")

	r.bus.Publish(bus.Message{
		Type: bus.EventPhaseChanged,
		Data: bus.PhaseChanged{
			Phase:        bus.PhaseStopped,
			Duration:     time.Since(shutdownStart),
			ServiceCount: serviceCount,
		},
		Critical: true,
	})

	return nil
}

// runStartupPhase handles the service startup phase
func (r *runner) runStartupPhase(ctx context.Context, cancel context.CancelFunc, tiers []bus.Tier, sigChan chan os.Signal, msgChan <-chan bus.Message) error {
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
			go func(svc bus.Service, files []string) {
				if err := r.worker.Acquire(ctx); err != nil {
					r.log.Warn().Err(err).Msgf("Failed to acquire worker for watch restart of '%s'", svc.Name)

					return
				}
				defer r.worker.Release()

				r.log.Info().Msgf("File change detected for service '%s': %v", svc.Name, files)
				r.service.Restart(ctx, svc)
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

	data, ok := msg.Data.(bus.Service)
	if !ok {
		return false
	}

	//nolint:exhaustive // only handling command types
	switch msg.Type {
	case bus.CommandStopService:
		r.service.Stop(data.ID)
	case bus.CommandStartService:
		go r.runWithWorker(ctx, data, r.service.Resume)
	case bus.CommandRestartService:
		go r.runWithWorker(ctx, data, r.service.Restart)
	}

	return false
}

// runWithWorker acquires a worker slot before running a service action
func (r *runner) runWithWorker(ctx context.Context, svc bus.Service, action func(context.Context, bus.Service)) {
	if err := r.worker.Acquire(ctx); err != nil {
		r.log.Warn().Err(err).Msgf("Failed to acquire worker for service '%s'", svc.Name)

		return
	}
	defer r.worker.Release()

	action(ctx, svc)
}

// startAllTiers starts services tier by tier
func (r *runner) startAllTiers(ctx context.Context, tiers []bus.Tier) {
	for tierIdx, tier := range tiers {
		if len(tier.Services) == 0 {
			continue
		}

		names := serviceNames(tier.Services)
		r.log.Info().Msgf("Starting tier '%s' (%d/%d) with services: %v", tier.Name, tierIdx+1, len(tiers), names)
		r.bus.Publish(bus.Message{
			Type:     bus.EventTierStarting,
			Data:     bus.TierStarting{Name: tier.Name},
			Critical: true,
		})

		tierStart := time.Now()
		failed := r.startTier(ctx, tier.Name, tier.Services)

		if len(failed) > 0 {
			r.log.Warn().Msgf("Tier '%s' partially failed: %d/%d services failed: %v", tier.Name, len(failed), len(tier.Services), failed)
		} else {
			r.log.Info().Msgf("Tier '%s' started successfully, all services ready", tier.Name)
			r.bus.Publish(bus.Message{
				Type: bus.EventTierReady,
				Data: bus.TierReady{
					Name:         tier.Name,
					Duration:     time.Since(tierStart),
					ServiceCount: len(tier.Services),
				},
				Critical: true,
			})
		}
	}
}

// startTier starts all services in a tier concurrently
func (r *runner) startTier(ctx context.Context, tier string, services []bus.Service) []string {
	failedChan := make(chan string, len(services))

	var wg sync.WaitGroup

	for _, ref := range services {
		wg.Add(1)

		go func(ref bus.Service) {
			defer wg.Done()

			if err := r.worker.Acquire(ctx); err != nil {
				r.log.Error().Err(err).Msgf("Failed to acquire worker for service '%s'", ref.Name)
				r.bus.Publish(bus.Message{
					Type:     bus.EventServiceFailed,
					Data:     bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: ref, Tier: tier}, Error: fmt.Errorf("%w: %w", errors.ErrFailedToAcquireWorker, err)},
					Critical: true,
				})

				failedChan <- ref.Name

				return
			}
			defer r.worker.Release()

			if err := r.service.Start(ctx, tier, ref); err != nil {
				failedChan <- ref.Name
			}
		}(ref)
	}

	wg.Wait()
	close(failedChan)

	failed := make([]string, 0, len(services))
	for name := range failedChan {
		failed = append(failed, name)
	}

	return failed
}

// shutdown stops all services in reverse order and returns the count of stopped services
func (r *runner) shutdown() int {
	entries := r.registry.SnapshotReverse()

	for _, e := range entries {
		r.registry.Detach(e.ID)
	}

	for _, e := range entries {
		r.service.Stop(e.ID)
	}

	r.registry.Wait()

	return len(entries)
}

// Stop resolves a profile and kills any processes running in service directories
func (r *runner) Stop(ctx context.Context, profile string) error {
	tiers, err := r.discovery.Resolve(profile)
	if err != nil {
		return fmt.Errorf("failed to resolve profile: %w", err)
	}

	services := collectDiscoveryServices(tiers)
	if len(services) == 0 {
		r.log.Warn().Msgf("No services found for profile '%s'", profile)

		return nil
	}

	dirs := r.resolveServiceDirs(services)
	if _, err := r.preflight.Cleanup(ctx, dirs); err != nil {
		r.log.Warn().Err(err).Msg("Preflight cleanup failed during stop")
	}

	return nil
}

// buildTiers converts discovery tiers to bus tiers with assigned UUIDs
func (r *runner) buildTiers(tiers []discovery.Tier) []bus.Tier {
	result := make([]bus.Tier, len(tiers))

	for i, tier := range tiers {
		refs := make([]bus.Service, len(tier.Services))
		for j, name := range tier.Services {
			refs[j] = bus.Service{
				ID:   uuid.NewString(),
				Name: name,
			}
		}

		result[i] = bus.Tier{
			ID:       uuid.NewString(),
			Name:     tier.Name,
			Services: refs,
		}
	}

	return result
}

// collectServiceNames extracts service names from bus tiers
func (r *runner) collectServiceNames(tiers []bus.Tier) []string {
	var names []string

	for _, tier := range tiers {
		for _, ref := range tier.Services {
			names = append(names, ref.Name)
		}
	}

	return names
}

func collectDiscoveryServices(tiers []discovery.Tier) []string {
	groups := make([][]string, len(tiers))
	for i, tier := range tiers {
		groups[i] = tier.Services
	}

	return slices.Concat(groups...)
}

func serviceNames(refs []bus.Service) []string {
	names := make([]string, len(refs))
	for i, ref := range refs {
		names[i] = ref.Name
	}

	return names
}

// startAPI starts the API server and publishes the started event
func (r *runner) startAPI(ctx context.Context) {
	if err := r.api.Start(ctx); err != nil {
		r.log.Warn().Err(err).Msg("Failed to start API server, continuing without it")

		return
	}

	r.apiStarted = true

	if r.cfg.APIEnabled() {
		r.bus.Publish(bus.Message{
			Type:     bus.EventAPIStarted,
			Data:     bus.APIStarted{Listen: r.cfg.API.Listen},
			Critical: true,
		})
	}
}

// stopAPI stops the API server and publishes the stopped event
func (r *runner) stopAPI() {
	if !r.apiStarted {
		return
	}

	//nolint:errcheck // best-effort cleanup on shutdown
	r.api.Stop()

	if r.cfg.APIEnabled() {
		r.bus.Publish(bus.Message{
			Type:     bus.EventAPIStopped,
			Data:     bus.APIStopped{},
			Critical: true,
		})
	}
}

// resolveServiceDirs resolves service names to their absolute directories
func (r *runner) resolveServiceDirs(services []string) map[string]string {
	wd, err := os.Getwd()
	if err != nil {
		r.log.Warn().Err(err).Msg("Failed to get working directory for preflight")

		return nil
	}

	dirs := make(map[string]string, len(services))

	for _, name := range services {
		cfg, exists := r.cfg.Services[name]
		if !exists {
			continue
		}

		dir := cfg.Dir
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(wd, dir)
		}

		dirs[name] = dir
	}

	return dirs
}
