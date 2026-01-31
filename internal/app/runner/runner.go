package runner

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"fuku/internal/app/bus"
	"fuku/internal/app/discovery"
	"fuku/internal/app/errors"
	"fuku/internal/app/logs"
	"fuku/internal/app/process"
	"fuku/internal/app/registry"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Runner defines the interface for service orchestration and execution
type Runner interface {
	Run(ctx context.Context, profile string) error
}

// runner implements the Runner interface
type runner struct {
	cfg       *config.Config
	discovery discovery.Discovery
	registry  registry.Registry
	service   Service
	guard     Guard
	pool      WorkerPool
	bus       bus.Bus
	log       logger.Logger
}

// NewRunner creates a new runner instance with the provided configuration and logger
func NewRunner(
	cfg *config.Config,
	disc discovery.Discovery,
	reg registry.Registry,
	service Service,
	guard Guard,
	pool WorkerPool,
	bus bus.Bus,
	log logger.Logger,
) Runner {
	return &runner{
		cfg:       cfg,
		discovery: disc,
		registry:  reg,
		service:   service,
		guard:     guard,
		pool:      pool,
		bus:       bus,
		log:       log.WithComponent("RUNNER"),
	}
}

// Run executes the specified profile by starting all services in dependency and tier order
func (r *runner) Run(ctx context.Context, profile string) error {
	logsServer := logs.NewServer(r.cfg, profile, r.log)
	if err := logsServer.Start(ctx); err != nil {
		r.log.Warn().Err(err).Msg("Failed to start logs server, continuing without it")
	} else {
		r.service.SetBroadcaster(logsServer)
		r.bus.SetBroadcaster(logsServer)

		defer func() {
			r.service.SetBroadcaster(nil)
			r.bus.SetBroadcaster(nil)
			logsServer.Stop()
		}()
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

	r.log.Debug().Msgf("Publishing EventProfileResolved: profile=%s, tiers=%d", profile, len(tierData))
	r.bus.Publish(bus.Message{
		Type:     bus.EventProfileResolved,
		Data:     bus.ProfileResolved{Profile: profile, Tiers: tierData},
		Critical: true,
	})
	r.log.Debug().Msg("EventProfileResolved published")

	var services []string
	for _, tier := range tiers {
		services = append(services, tier.Services...)
	}

	if len(services) == 0 {
		r.bus.Publish(bus.Message{
			Type:     bus.EventPhaseChanged,
			Data:     bus.PhaseChanged{Phase: bus.PhaseStopped},
			Critical: true,
		})
		r.log.Warn().Msgf("No services found for profile '%s'. Nothing to run.", profile)

		return nil
	}

	r.log.Info().Msgf("Starting services in profile '%s': %v", profile, services)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	msgChan := r.bus.Subscribe(ctx)

	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	startupErr := r.runStartupPhase(ctx, cancel, tiers, r.registry, sigChan, msgChan)
	if startupErr != nil {
		r.bus.Publish(bus.Message{
			Type:     bus.EventPhaseChanged,
			Data:     bus.PhaseChanged{Phase: bus.PhaseStopped},
			Critical: true,
		})

		return startupErr
	}

	r.bus.Publish(bus.Message{
		Type:     bus.EventPhaseChanged,
		Data:     bus.PhaseChanged{Phase: bus.PhaseRunning},
		Critical: true,
	})

	r.runServicePhase(ctx, cancel, sigChan, r.registry, msgChan)

	r.bus.Publish(bus.Message{
		Type:     bus.EventPhaseChanged,
		Data:     bus.PhaseChanged{Phase: bus.PhaseStopping},
		Critical: true,
	})

	r.shutdown(r.registry)
	r.log.Info().Msg("All services stopped")

	r.bus.Publish(bus.Message{
		Type:     bus.EventPhaseChanged,
		Data:     bus.PhaseChanged{Phase: bus.PhaseStopped},
		Critical: true,
	})

	return nil
}

// runStartupPhase handles the service startup phase and waits for completion or interruption
func (r *runner) runStartupPhase(ctx context.Context, cancel context.CancelFunc, tiers []discovery.Tier, registry registry.Registry, sigChan chan os.Signal, msgChan <-chan bus.Message) error {
	startupDone := make(chan struct{}, 1)

	go func() {
		r.startAllTiers(ctx, tiers, registry)

		startupDone <- struct{}{}
	}()

	for {
		select {
		case <-startupDone:
			r.log.Info().Msg("Startup phase complete, waiting for signals...")

			return nil
		case sig := <-sigChan:
			r.bus.Publish(bus.Message{
				Type:     bus.EventSignal,
				Data:     bus.Signal{Name: sig.String()},
				Critical: true,
			})
			r.log.Info().Msgf("Received signal %s during startup, shutting down services...", sig)
			cancel()
			<-startupDone
			r.shutdown(registry)

			return fmt.Errorf("%w: signal %s", errors.ErrStartupInterrupted, sig)
		case <-ctx.Done():
			r.log.Info().Msg("Context cancelled during startup, shutting down services...")
			cancel()
			<-startupDone
			r.shutdown(registry)

			return ctx.Err()
		case msg, ok := <-msgChan:
			if !ok {
				r.log.Info().Msg("Message channel closed during startup, shutting down services...")
				cancel()
				<-startupDone
				r.shutdown(registry)

				return errors.ErrCommandChannelClosed
			}

			if msg.Type == bus.CommandStopAll {
				r.log.Info().Msg("Received StopAll command during startup, shutting down services...")
				cancel()
				<-startupDone
				r.shutdown(registry)

				return fmt.Errorf("%w: StopAll command", errors.ErrStartupInterrupted)
			}

			r.log.Debug().Msgf("Handling message during startup: %v", msg.Type)
			_ = r.handleCommand(ctx, msg, registry)
		}
	}
}

// runServicePhase runs the main event loop handling signals and commands
func (r *runner) runServicePhase(ctx context.Context, cancel context.CancelFunc, sigChan chan os.Signal, registry registry.Registry, msgChan <-chan bus.Message) {
	for {
		select {
		case sig := <-sigChan:
			r.bus.Publish(bus.Message{
				Type:     bus.EventSignal,
				Data:     bus.Signal{Name: sig.String()},
				Critical: true,
			})
			r.log.Info().Msgf("Received signal %s, shutting down services...", sig)
			cancel()

			return
		case <-ctx.Done():
			r.log.Info().Msg("Context cancelled, shutting down services...")
			return
		case msg, ok := <-msgChan:
			if !ok {
				return
			}

			if r.handleMessage(ctx, msg, registry) {
				cancel()
				return
			}
		}
	}
}

// handleMessage processes a message and returns true if shutdown is requested
func (r *runner) handleMessage(ctx context.Context, msg bus.Message, registry registry.Registry) bool {
	switch msg.Type {
	case bus.EventWatchTriggered:
		if data, ok := msg.Data.(bus.WatchTriggered); ok {
			go r.handleWatchEvent(ctx, data.Service, data.ChangedFiles, registry)
		}
	default:
		return r.handleCommand(ctx, msg, registry)
	}

	return false
}

// handleCommand processes a command and returns true if shutdown is requested
func (r *runner) handleCommand(ctx context.Context, msg bus.Message, registry registry.Registry) bool {
	switch msg.Type {
	case bus.CommandStopService:
		data, ok := msg.Data.(bus.Payload)
		if !ok {
			r.log.Error().Msg("Invalid StopService command data")
			return false
		}

		r.stopService(data.Name, registry)

	case bus.CommandRestartService:
		data, ok := msg.Data.(bus.Payload)
		if !ok {
			r.log.Error().Msg("Invalid RestartService command data")
			return false
		}

		r.restartService(ctx, data.Name, registry)

	case bus.CommandStopAll:
		r.log.Info().Msg("Received StopAll command, shutting down all services...")
		return true
	}

	return false
}

// handleWatchEvent processes a file watch event and triggers service restart
func (r *runner) handleWatchEvent(ctx context.Context, service string, changedFiles []string, registry registry.Registry) {
	r.log.Info().Msgf("File change detected for service '%s': %v", service, changedFiles)

	if !r.guard.Lock(service) {
		r.log.Info().Msgf("Service '%s' restart already in progress, skipping", service)
		return
	}

	r.restartWatchedService(ctx, service, registry)
}

// isWatchedService returns true if the service has watch configuration
func (r *runner) isWatchedService(service string) bool {
	serviceCfg, exists := r.cfg.Services[service]
	if !exists {
		return false
	}

	return serviceCfg.Watch != nil
}

// watchProcess creates a goroutine that monitors process lifecycle and publishes events
func (r *runner) watchProcess(proc process.Process, registry registry.Registry) {
	go func() {
		<-proc.Done()

		result := registry.Remove(proc.Name(), proc)
		if !result.Removed {
			return
		}

		if result.UnexpectedExit {
			r.log.Info().Msgf("Service '%s' exited unexpectedly", proc.Name())

			if r.isWatchedService(proc.Name()) {
				r.guard.Unlock(proc.Name())

				r.bus.Publish(bus.Message{
					Type:     bus.EventServiceFailed,
					Data:     bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: proc.Name(), Tier: result.Tier}, Error: errors.ErrUnexpectedExit},
					Critical: true,
				})
			} else {
				r.bus.Publish(bus.Message{
					Type:     bus.EventServiceStopped,
					Data:     bus.ServiceStopped{ServiceEvent: bus.ServiceEvent{Service: proc.Name(), Tier: result.Tier}},
					Critical: true,
				})
			}
		}
	}()
}

// stopService stops a single service by name
func (r *runner) stopService(serviceName string, registry registry.Registry) {
	lookup := registry.Get(serviceName)
	if !lookup.Exists {
		r.log.Warn().Msgf("Service '%s' not found in registry", serviceName)
		r.bus.Publish(bus.Message{
			Type:     bus.EventServiceFailed,
			Data:     bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: serviceName, Tier: ""}, Error: errors.ErrServiceNotInRegistry},
			Critical: true,
		})

		return
	}

	tier := lookup.Tier

	r.bus.Publish(bus.Message{
		Type:     bus.EventServiceStopping,
		Data:     bus.ServiceStopping{ServiceEvent: bus.ServiceEvent{Service: serviceName, Tier: tier}},
		Critical: true,
	})

	registry.Detach(serviceName)

	r.log.Info().Msgf("Stopping service '%s' by command", serviceName)
	r.service.Stop(lookup.Proc)
	<-lookup.Proc.Done()

	registry.Remove(serviceName, lookup.Proc)

	r.log.Info().Msgf("Service '%s' stopped", serviceName)
	r.bus.Publish(bus.Message{
		Type:     bus.EventServiceStopped,
		Data:     bus.ServiceStopped{ServiceEvent: bus.ServiceEvent{Service: serviceName, Tier: tier}},
		Critical: true,
	})
}

// restartService stops and starts a service, or just starts if not running (used for manual restarts via commands)
func (r *runner) restartService(ctx context.Context, serviceName string, registry registry.Registry) {
	r.log.Debug().Msgf("restartService called for '%s'", serviceName)

	serviceCfg, exists := r.cfg.Services[serviceName]
	if !exists {
		r.log.Error().Msgf("Service configuration for '%s' not found", serviceName)
		r.bus.Publish(bus.Message{
			Type:     bus.EventServiceFailed,
			Data:     bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: serviceName, Tier: ""}, Error: errors.ErrServiceNotFound},
			Critical: true,
		})

		return
	}

	tier := serviceCfg.Tier
	if tier == "" {
		tier = config.Default
	}

	r.bus.Publish(bus.Message{
		Type:     bus.EventServiceRestarting,
		Data:     bus.ServiceRestarting{ServiceEvent: bus.ServiceEvent{Service: serviceName, Tier: tier}},
		Critical: true,
	})

	lookup := registry.Get(serviceName)

	if lookup.Exists {
		r.log.Info().Msgf("Stopping service '%s' before restart", serviceName)
		registry.Detach(serviceName)
		r.service.Stop(lookup.Proc)
		<-lookup.Proc.Done()
		registry.Remove(serviceName, lookup.Proc)
		r.log.Info().Msgf("Service '%s' stopped, starting new instance", serviceName)
	} else {
		r.log.Info().Msgf("Starting stopped service '%s'", serviceName)
	}

	newProc, err := r.startServiceWithRetry(ctx, serviceName, tier, serviceCfg)
	if err != nil {
		r.log.Error().Err(err).Msgf("Failed to restart service '%s'", serviceName)
		r.bus.Publish(bus.Message{
			Type:     bus.EventServiceFailed,
			Data:     bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: serviceName, Tier: tier}, Error: err},
			Critical: true,
		})

		return
	}

	registry.Add(serviceName, newProc, tier)
	r.watchProcess(newProc, registry)
}

// restartWatchedService handles restart for watched services with FSM state management
func (r *runner) restartWatchedService(ctx context.Context, serviceName string, registry registry.Registry) {
	r.log.Debug().Msgf("restartWatchedService called for '%s'", serviceName)

	serviceCfg, exists := r.cfg.Services[serviceName]
	if !exists {
		r.log.Error().Msgf("Service configuration for '%s' not found", serviceName)
		r.guard.Unlock(serviceName)
		r.bus.Publish(bus.Message{
			Type:     bus.EventServiceFailed,
			Data:     bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: serviceName, Tier: ""}, Error: errors.ErrServiceNotFound},
			Critical: true,
		})

		return
	}

	tier := serviceCfg.Tier
	if tier == "" {
		tier = config.Default
	}

	r.bus.Publish(bus.Message{
		Type:     bus.EventServiceRestarting,
		Data:     bus.ServiceRestarting{ServiceEvent: bus.ServiceEvent{Service: serviceName, Tier: tier}},
		Critical: true,
	})

	lookup := registry.Get(serviceName)

	if lookup.Exists {
		r.log.Info().Msgf("Stopping service '%s' before restart", serviceName)
		registry.Detach(serviceName)
		r.service.Stop(lookup.Proc)
		<-lookup.Proc.Done()
		registry.Remove(serviceName, lookup.Proc)
		r.log.Info().Msgf("Service '%s' stopped, starting new instance", serviceName)
	} else {
		r.log.Info().Msgf("Starting stopped service '%s'", serviceName)
	}

	newProc, err := r.startServiceOnce(ctx, serviceName, tier, serviceCfg)
	if err != nil {
		r.log.Error().Err(err).Msgf("Failed to restart service '%s'", serviceName)
		r.guard.Unlock(serviceName)
		r.bus.Publish(bus.Message{
			Type:     bus.EventServiceFailed,
			Data:     bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: serviceName, Tier: tier}, Error: err},
			Critical: true,
		})

		return
	}

	registry.Add(serviceName, newProc, tier)
	r.guard.Unlock(serviceName)
	r.watchProcess(newProc, registry)
}

// startTier starts all services in a tier concurrently and returns failed service names
func (r *runner) startTier(ctx context.Context, tierName string, tierServices []string, registry registry.Registry) []string {
	failedChan := make(chan string, len(tierServices))
	procChan := make(chan process.Process, len(tierServices))

	var tierWg sync.WaitGroup

	for _, serviceName := range tierServices {
		tierWg.Add(1)

		go func(name string) {
			defer tierWg.Done()

			if err := r.pool.Acquire(ctx); err != nil {
				r.log.Error().Err(err).Msgf("Failed to acquire worker for service '%s'", name)
				r.bus.Publish(bus.Message{
					Type:     bus.EventServiceFailed,
					Data:     bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: name, Tier: tierName}, Error: fmt.Errorf("%w: %w", errors.ErrFailedToAcquireWorker, err)},
					Critical: true,
				})

				failedChan <- name

				return
			}
			defer r.pool.Release()

			srv := r.cfg.Services[name]

			proc, err := r.startServiceWithRetry(ctx, name, tierName, srv)
			if err != nil {
				r.log.Error().Err(err).Msgf("Failed to start service '%s'", name)
				r.bus.Publish(bus.Message{
					Type:     bus.EventServiceFailed,
					Data:     bus.ServiceFailed{ServiceEvent: bus.ServiceEvent{Service: name, Tier: tierName}, Error: err},
					Critical: true,
				})

				failedChan <- name

				return
			}

			procChan <- proc
		}(serviceName)
	}

	tierWg.Wait()
	close(failedChan)
	close(procChan)

	for proc := range procChan {
		registry.Add(proc.Name(), proc, tierName)
		r.watchProcess(proc, registry)
	}

	failedServices := make([]string, 0, len(tierServices))
	for name := range failedChan {
		failedServices = append(failedServices, name)
	}

	return failedServices
}

// startServiceOnce attempts to start a service once without retries (used for watched service restarts)
func (r *runner) startServiceOnce(ctx context.Context, name string, tierName string, service *config.Service) (process.Process, error) {
	startTime := time.Now()

	proc, err := r.service.Start(ctx, name, service)
	if err != nil {
		return nil, err
	}

	pid := 0
	if proc.Cmd() != nil && proc.Cmd().Process != nil {
		pid = proc.Cmd().Process.Pid
	}

	r.bus.Publish(bus.Message{
		Type:     bus.EventServiceStarting,
		Data:     bus.ServiceStarting{ServiceEvent: bus.ServiceEvent{Service: name, Tier: tierName}, Attempt: 1, PID: pid},
		Critical: true,
	})

	if service.Readiness != nil {
		select {
		case err := <-proc.Ready():
			if err != nil {
				r.service.Stop(proc)
				return nil, fmt.Errorf("readiness check failed: %w", err)
			}

			r.bus.Publish(bus.Message{
				Type:     bus.EventServiceReady,
				Data:     bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: name, Tier: tierName}, Duration: time.Since(startTime)},
				Critical: true,
			})

			return proc, nil
		case <-ctx.Done():
			r.service.Stop(proc)
			return nil, ctx.Err()
		}
	}

	r.bus.Publish(bus.Message{
		Type:     bus.EventServiceReady,
		Data:     bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: name, Tier: tierName}, Duration: time.Since(startTime)},
		Critical: true,
	})

	return proc, nil
}

// startServiceWithRetry attempts to start a service with configurable retries
func (r *runner) startServiceWithRetry(ctx context.Context, name string, tierName string, service *config.Service) (process.Process, error) {
	var lastErr error

	for attempt := 0; attempt < r.cfg.Retry.Attempts; attempt++ {
		if attempt > 0 {
			r.log.Info().Msgf("Retrying service '%s' (attempt %d/%d)", name, attempt+1, r.cfg.Retry.Attempts)

			select {
			case <-time.After(r.cfg.Retry.Backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		startTime := time.Now()

		proc, err := r.service.Start(ctx, name, service)
		if err != nil {
			lastErr = err

			continue
		}

		pid := 0
		if proc.Cmd() != nil && proc.Cmd().Process != nil {
			pid = proc.Cmd().Process.Pid
		}

		r.bus.Publish(bus.Message{
			Type:     bus.EventServiceStarting,
			Data:     bus.ServiceStarting{ServiceEvent: bus.ServiceEvent{Service: name, Tier: tierName}, Attempt: attempt + 1, PID: pid},
			Critical: true,
		})

		if service.Readiness != nil {
			select {
			case err := <-proc.Ready():
				if err != nil {
					lastErr = fmt.Errorf("readiness check failed: %w", err)

					r.service.Stop(proc)

					continue
				}

				r.bus.Publish(bus.Message{
					Type:     bus.EventServiceReady,
					Data:     bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: name, Tier: tierName}, Duration: time.Since(startTime)},
					Critical: true,
				})

				return proc, nil
			case <-ctx.Done():
				r.service.Stop(proc)
				return nil, ctx.Err()
			}
		} else {
			r.bus.Publish(bus.Message{
				Type:     bus.EventServiceReady,
				Data:     bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: name, Tier: tierName}, Duration: time.Since(startTime)},
				Critical: true,
			})

			return proc, nil
		}
	}

	return nil, fmt.Errorf("%w after %d attempts: %w", errors.ErrMaxRetriesExceeded, r.cfg.Retry.Attempts, lastErr)
}

// startAllTiers starts services tier by tier in order
func (r *runner) startAllTiers(ctx context.Context, tiers []discovery.Tier, registry registry.Registry) {
	for tierIdx, tier := range tiers {
		if len(tier.Services) > 0 {
			r.bus.Publish(bus.Message{
				Type:     bus.EventTierStarting,
				Data:     bus.TierStarting{Name: tier.Name, Index: tierIdx + 1, Total: len(tiers)},
				Critical: true,
			})
			r.log.Info().Msgf("Starting tier '%s' (%d/%d) with services: %v", tier.Name, tierIdx+1, len(tiers), tier.Services)
		}

		failedServices := r.startTier(ctx, tier.Name, tier.Services, registry)

		if len(failedServices) > 0 {
			r.log.Warn().Msgf("Tier '%s' partially failed: %d/%d services failed: %v", tier.Name, len(failedServices), len(tier.Services), failedServices)
		} else if len(tier.Services) > 0 {
			r.bus.Publish(bus.Message{
				Type:     bus.EventTierReady,
				Data:     bus.Payload{Name: tier.Name},
				Critical: true,
			})
			r.log.Info().Msgf("Tier '%s' started successfully, all services ready", tier.Name)
		}
	}
}

// shutdown stops all services in reverse order and waits for completion
func (r *runner) shutdown(registry registry.Registry) {
	processes := registry.SnapshotReverse()

	for _, proc := range processes {
		registry.Detach(proc.Name())
	}

	for _, proc := range processes {
		r.service.Stop(proc)
	}

	registry.Wait()
}
