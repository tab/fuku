package runner

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"fuku/internal/app/runtime"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Runner defines the interface for service orchestration and execution
type Runner interface {
	Run(ctx context.Context, profile string) error
}

type runner struct {
	cfg       *config.Config
	discovery Discovery
	service   Service
	pool      WorkerPool
	log       logger.Logger
	event     runtime.EventBus
	command   runtime.CommandBus
}

// NewRunner creates a new runner instance with the provided configuration and logger
func NewRunner(
	cfg *config.Config,
	discovery Discovery,
	service Service,
	pool WorkerPool,
	log logger.Logger,
	event runtime.EventBus,
	command runtime.CommandBus,
) Runner {
	return &runner{
		cfg:       cfg,
		discovery: discovery,
		service:   service,
		pool:      pool,
		log:       log,
		event:     event,
		command:   command,
	}
}

// Run executes the specified profile by starting all services in dependency and tier order
func (r *runner) Run(ctx context.Context, profile string) error {
	r.event.Publish(runtime.Event{
		Type:     runtime.EventPhaseChanged,
		Data:     runtime.PhaseChangedData{Phase: runtime.PhaseStartup},
		Critical: true,
	})

	commandChan := r.command.Subscribe(ctx)

	tiers, err := r.discovery.Resolve(profile)
	if err != nil {
		return fmt.Errorf("failed to resolve profile: %w", err)
	}

	tierData := make([]runtime.TierData, len(tiers))
	for i, tier := range tiers {
		tierData[i] = runtime.TierData{Name: tier.Name, Services: tier.Services}
	}

	r.log.Debug().Msgf("[RUNNER] Publishing EventProfileResolved: profile=%s, tiers=%d", profile, len(tierData))
	r.event.Publish(runtime.Event{
		Type:     runtime.EventProfileResolved,
		Data:     runtime.ProfileResolvedData{Profile: profile, Tiers: tierData},
		Critical: true,
	})
	r.log.Debug().Msg("[RUNNER] EventProfileResolved published")

	var services []string
	for _, tier := range tiers {
		services = append(services, tier.Services...)
	}

	r.log.Info().Msgf("Starting services in profile '%s': %v", profile, services)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	processes := make([]Process, 0, len(services))

	var processesMu sync.Mutex

	var wg sync.WaitGroup

	serviceMap := make(map[string]Process)

	var serviceMapMu sync.Mutex

	startupErr := r.runStartupPhase(ctx, tiers, &processes, &processesMu, &serviceMap, &serviceMapMu, &wg, sigChan, commandChan)
	if startupErr != nil {
		r.event.Publish(runtime.Event{
			Type:     runtime.EventPhaseChanged,
			Data:     runtime.PhaseChangedData{Phase: runtime.PhaseStopped},
			Critical: true,
		})

		return startupErr
	}

	r.event.Publish(runtime.Event{
		Type:     runtime.EventPhaseChanged,
		Data:     runtime.PhaseChangedData{Phase: runtime.PhaseRunning},
		Critical: true,
	})

	r.runServicePhase(ctx, sigChan, &serviceMap, &serviceMapMu, commandChan)

	r.event.Publish(runtime.Event{
		Type:     runtime.EventPhaseChanged,
		Data:     runtime.PhaseChangedData{Phase: runtime.PhaseStopping},
		Critical: true,
	})

	r.shutdown(processes, &processesMu, &serviceMap, &serviceMapMu, &wg)
	r.log.Info().Msg("All services stopped")

	r.event.Publish(runtime.Event{
		Type:     runtime.EventPhaseChanged,
		Data:     runtime.PhaseChangedData{Phase: runtime.PhaseStopped},
		Critical: true,
	})

	return nil
}

func (r *runner) runStartupPhase(ctx context.Context, tiers []Tier, processes *[]Process, processesMu *sync.Mutex, serviceMap *map[string]Process, serviceMapMu *sync.Mutex, wg *sync.WaitGroup, sigChan chan os.Signal, commandChan <-chan runtime.Command) error {
	startupDone := make(chan error, 1)

	go func() {
		startupDone <- r.startAllTiers(ctx, tiers, processes, processesMu, serviceMap, serviceMapMu, wg)
	}()

	for {
		select {
		case err := <-startupDone:
			if err != nil {
				r.log.Error().Err(err).Msg("Failed to start services")
				r.shutdown(*processes, processesMu, serviceMap, serviceMapMu, wg)

				return err
			}

			r.log.Info().Msg("All services started successfully, waiting for signals...")

			return nil
		case sig := <-sigChan:
			r.event.Publish(runtime.Event{
				Type:     runtime.EventSignalCaught,
				Data:     runtime.SignalCaughtData{Signal: sig.String()},
				Critical: true,
			})
			r.log.Info().Msgf("Received signal %s during startup, shutting down services...", sig)
			r.shutdown(*processes, processesMu, serviceMap, serviceMapMu, wg)

			return fmt.Errorf("startup interrupted by signal %s", sig)
		case <-ctx.Done():
			r.log.Info().Msg("Context cancelled during startup, shutting down services...")
			r.shutdown(*processes, processesMu, serviceMap, serviceMapMu, wg)

			return ctx.Err()
		case cmd, ok := <-commandChan:
			if !ok {
				r.log.Info().Msg("Command channel closed during startup, shutting down services...")
				r.shutdown(*processes, processesMu, serviceMap, serviceMapMu, wg)

				return fmt.Errorf("command channel closed")
			}

			if cmd.Type == runtime.CommandStopAll {
				r.log.Info().Msg("Received StopAll command during startup, shutting down services...")
				r.shutdown(*processes, processesMu, serviceMap, serviceMapMu, wg)

				return fmt.Errorf("startup interrupted by StopAll command")
			}
		}
	}
}

func (r *runner) runServicePhase(ctx context.Context, sigChan chan os.Signal, serviceMap *map[string]Process, serviceMapMu *sync.Mutex, commandChan <-chan runtime.Command) {
	for {
		select {
		case sig := <-sigChan:
			r.event.Publish(runtime.Event{
				Type:     runtime.EventSignalCaught,
				Data:     runtime.SignalCaughtData{Signal: sig.String()},
				Critical: true,
			})
			r.log.Info().Msgf("Received signal %s, shutting down services...", sig)

			return
		case <-ctx.Done():
			r.log.Info().Msg("Context cancelled, shutting down services...")
			return
		case cmd, ok := <-commandChan:
			if !ok {
				return
			}

			if r.handleCommand(ctx, cmd, serviceMap, serviceMapMu) {
				return
			}
		}
	}
}

func (r *runner) handleCommand(ctx context.Context, cmd runtime.Command, serviceMap *map[string]Process, serviceMapMu *sync.Mutex) bool {
	switch cmd.Type {
	case runtime.CommandStopService:
		data, ok := cmd.Data.(runtime.StopServiceData)
		if !ok {
			r.log.Error().Msg("Invalid StopService command data")
			return false
		}

		r.stopService(data.Service, serviceMap, serviceMapMu)

	case runtime.CommandRestartService:
		data, ok := cmd.Data.(runtime.RestartServiceData)
		if !ok {
			r.log.Error().Msg("Invalid RestartService command data")
			return false
		}

		r.restartService(ctx, data.Service, serviceMap, serviceMapMu)

	case runtime.CommandStopAll:
		r.log.Info().Msg("Received StopAll command, shutting down all services...")
		return true
	}

	return false
}

func (r *runner) stopService(serviceName string, serviceMap *map[string]Process, serviceMapMu *sync.Mutex) {
	serviceMapMu.Lock()

	proc, exists := (*serviceMap)[serviceName]
	if !exists {
		serviceMapMu.Unlock()
		r.log.Warn().Msgf("Service '%s' not found", serviceName)

		return
	}

	delete(*serviceMap, serviceName)
	serviceMapMu.Unlock()

	r.log.Info().Msgf("Stopping service '%s' by command", serviceName)
	r.service.Stop(proc)
}

func (r *runner) restartService(ctx context.Context, serviceName string, serviceMap *map[string]Process, serviceMapMu *sync.Mutex) {
	serviceMapMu.Lock()

	proc, exists := (*serviceMap)[serviceName]

	serviceMapMu.Unlock()

	if exists {
		r.log.Info().Msgf("Stopping service '%s' before restart", serviceName)
		r.service.Stop(proc)
	} else {
		r.log.Info().Msgf("Starting stopped service '%s'", serviceName)
	}

	serviceCfg, exists := r.cfg.Services[serviceName]
	if !exists {
		r.log.Error().Msgf("Service configuration for '%s' not found", serviceName)
		return
	}

	tier := serviceCfg.Tier
	if tier == "" {
		tier = config.Default
	}

	newProc, err := r.startServiceWithRetry(ctx, serviceName, tier, serviceCfg)
	if err != nil {
		r.log.Error().Err(err).Msgf("Failed to restart service '%s'", serviceName)
		r.event.Publish(runtime.Event{
			Type: runtime.EventServiceFailed,
			Data: runtime.ServiceFailedData{Service: serviceName, Tier: tier, Error: err},
		})

		return
	}

	serviceMapMu.Lock()

	(*serviceMap)[serviceName] = newProc

	serviceMapMu.Unlock()

	go func() {
		<-newProc.Done()
		r.log.Info().Msgf("Service '%s' stopped", newProc.Name())
		r.event.Publish(runtime.Event{
			Type: runtime.EventServiceStopped,
			Data: runtime.ServiceStoppedData{Service: newProc.Name(), Tier: tier},
		})
	}()
}

func (r *runner) startTier(ctx context.Context, tierName string, tierServices []string, wg *sync.WaitGroup, serviceMap *map[string]Process, serviceMapMu *sync.Mutex) ([]Process, error) {
	processes := make([]Process, 0, len(tierServices))
	errChan := make(chan error, len(tierServices))
	procChan := make(chan Process, len(tierServices))

	var tierWg sync.WaitGroup

	for _, serviceName := range tierServices {
		tierWg.Add(1)

		go func(name string) {
			defer tierWg.Done()

			r.pool.Acquire()
			defer r.pool.Release()

			srv := r.cfg.Services[name]

			proc, err := r.startServiceWithRetry(ctx, name, tierName, srv)
			if err != nil {
				r.event.Publish(runtime.Event{
					Type: runtime.EventServiceFailed,
					Data: runtime.ServiceFailedData{Service: name, Tier: tierName, Error: err},
				})

				errChan <- fmt.Errorf("service '%s': %w", name, err)

				return
			}

			procChan <- proc
		}(serviceName)
	}

	tierWg.Wait()
	close(errChan)
	close(procChan)

	for proc := range procChan {
		processes = append(processes, proc)

		serviceMapMu.Lock()

		(*serviceMap)[proc.Name()] = proc

		serviceMapMu.Unlock()

		wg.Add(1)

		go func(p Process, tier string) {
			defer wg.Done()

			<-p.Done()
			r.log.Info().Msgf("Service '%s' stopped", p.Name())
			r.event.Publish(runtime.Event{
				Type: runtime.EventServiceStopped,
				Data: runtime.ServiceStoppedData{Service: p.Name(), Tier: tier},
			})
		}(proc, tierName)
	}

	select {
	case err := <-errChan:
		return processes, err
	default:
		return processes, nil
	}
}

func (r *runner) startServiceWithRetry(ctx context.Context, name string, tierName string, service *config.Service) (Process, error) {
	var lastErr error

	for attempt := 0; attempt < config.RetryAttempt; attempt++ {
		if attempt > 0 {
			r.event.Publish(runtime.Event{
				Type: runtime.EventRetryScheduled,
				Data: runtime.RetryScheduledData{Service: name, Attempt: attempt + 1, MaxAttempts: config.RetryAttempt},
			})
			r.log.Info().Msgf("Retrying service '%s' (attempt %d/%d)", name, attempt+1, config.RetryAttempt)
			time.Sleep(config.RetryBackoff)
		}

		startTime := time.Now()

		proc, err := r.service.Start(ctx, name, service)
		if err != nil {
			lastErr = err

			continue
		}

		r.event.Publish(runtime.Event{
			Type: runtime.EventServiceStarting,
			Data: runtime.ServiceStartingData{Service: name, Tier: tierName, Attempt: attempt + 1, PID: proc.Cmd().Process.Pid},
		})

		if service.Readiness != nil {
			select {
			case err := <-proc.Ready():
				if err != nil {
					lastErr = fmt.Errorf("readiness check failed: %w", err)

					r.service.Stop(proc)

					continue
				}

				r.event.Publish(runtime.Event{
					Type: runtime.EventServiceReady,
					Data: runtime.ServiceReadyData{Service: name, Tier: tierName, Duration: time.Since(startTime)},
				})

				return proc, nil
			case <-ctx.Done():
				r.service.Stop(proc)
				return nil, ctx.Err()
			}
		} else {
			r.event.Publish(runtime.Event{
				Type: runtime.EventServiceReady,
				Data: runtime.ServiceReadyData{Service: name, Tier: tierName, Duration: time.Since(startTime)},
			})

			return proc, nil
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", config.RetryAttempt, lastErr)
}

func (r *runner) startAllTiers(ctx context.Context, tiers []Tier, processes *[]Process, processesMu *sync.Mutex, serviceMap *map[string]Process, serviceMapMu *sync.Mutex, wg *sync.WaitGroup) error {
	for tierIdx, tier := range tiers {
		if len(tier.Services) > 0 {
			r.event.Publish(runtime.Event{
				Type: runtime.EventTierStarting,
				Data: runtime.TierStartingData{Name: tier.Name, Index: tierIdx + 1, Total: len(tiers)},
			})
			r.log.Info().Msgf("Starting tier '%s' (%d/%d) with services: %v", tier.Name, tierIdx+1, len(tiers), tier.Services)
		}

		tierProcs, err := r.startTier(ctx, tier.Name, tier.Services, wg, serviceMap, serviceMapMu)

		processesMu.Lock()

		*processes = append(*processes, tierProcs...)

		processesMu.Unlock()

		if err != nil {
			return err
		}

		if len(tier.Services) > 0 {
			r.event.Publish(runtime.Event{
				Type: runtime.EventTierReady,
				Data: runtime.TierReadyData{Name: tier.Name},
			})
			r.log.Info().Msgf("Tier '%s' started successfully, all services ready", tier.Name)
		}
	}

	return nil
}

func (r *runner) shutdown(processes []Process, processesMu *sync.Mutex, serviceMap *map[string]Process, serviceMapMu *sync.Mutex, wg *sync.WaitGroup) {
	processesMu.Lock()
	serviceMapMu.Lock()
	r.stopAllProcesses(processes)

	for _, proc := range *serviceMap {
		r.service.Stop(proc)
	}

	serviceMapMu.Unlock()
	processesMu.Unlock()
	wg.Wait()
}

func (r *runner) stopAllProcesses(processes []Process) {
	for i := range processes {
		idx := len(processes) - 1 - i
		r.service.Stop(processes[idx])
	}
}
