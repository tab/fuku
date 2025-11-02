package runner

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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
}

// NewRunner creates a new runner instance with the provided configuration and logger
func NewRunner(
	cfg *config.Config,
	discovery Discovery,
	service Service,
	pool WorkerPool,
	log logger.Logger,
) Runner {
	return &runner{
		cfg:       cfg,
		discovery: discovery,
		service:   service,
		pool:      pool,
		log:       log,
	}
}

// Run executes the specified profile by starting all services in dependency and tier order
func (r *runner) Run(ctx context.Context, profile string) error {
	tiers, err := r.discovery.Resolve(profile)
	if err != nil {
		return fmt.Errorf("failed to resolve profile: %w", err)
	}

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

	startupErr := r.runStartupPhase(ctx, tiers, &processes, &processesMu, &wg, sigChan)
	if startupErr != nil {
		return startupErr
	}

	r.runServicePhase(ctx, sigChan)

	r.shutdown(processes, &processesMu, &wg)
	r.log.Info().Msg("All services stopped")
	return nil
}

func (r *runner) runStartupPhase(ctx context.Context, tiers []Tier, processes *[]Process, processesMu *sync.Mutex, wg *sync.WaitGroup, sigChan chan os.Signal) error {
	startupDone := make(chan error, 1)

	go func() {
		startupDone <- r.startAllTiers(ctx, tiers, processes, processesMu, wg)
	}()

	select {
	case err := <-startupDone:
		if err != nil {
			r.log.Error().Err(err).Msg("Failed to start services")
			r.shutdown(*processes, processesMu, wg)
			return err
		}
		r.log.Info().Msg("All services started successfully, waiting for signals...")
		return nil
	case sig := <-sigChan:
		r.log.Info().Msgf("Received signal %s during startup, shutting down services...", sig)
		r.shutdown(*processes, processesMu, wg)
		return nil
	case <-ctx.Done():
		r.log.Info().Msg("Context cancelled during startup, shutting down services...")
		r.shutdown(*processes, processesMu, wg)
		return ctx.Err()
	}
}

func (r *runner) runServicePhase(ctx context.Context, sigChan chan os.Signal) {
	select {
	case sig := <-sigChan:
		r.log.Info().Msgf("Received signal %s, shutting down services...", sig)
	case <-ctx.Done():
		r.log.Info().Msg("Context cancelled, shutting down services...")
	}
}

func (r *runner) startTier(ctx context.Context, tierServices []string, wg *sync.WaitGroup) ([]Process, error) {
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
			proc, err := r.startServiceWithRetry(ctx, name, srv)
			if err != nil {
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
		wg.Add(1)
		go func(p Process) {
			defer wg.Done()
			<-p.Done()
			r.log.Info().Msgf("Service '%s' stopped", p.Name())
		}(proc)
	}

	select {
	case err := <-errChan:
		return processes, err
	default:
		return processes, nil
	}
}

func (r *runner) startServiceWithRetry(ctx context.Context, name string, service *config.Service) (Process, error) {
	var lastErr error
	for attempt := 0; attempt < config.RetryAttempt; attempt++ {
		if attempt > 0 {
			r.log.Info().Msgf("Retrying service '%s' (attempt %d/%d)", name, attempt+1, config.RetryAttempt)
			time.Sleep(config.RetryBackoff)
		}

		proc, err := r.service.Start(ctx, name, service)
		if err != nil {
			lastErr = err
			continue
		}

		if service.Readiness != nil {
			select {
			case err := <-proc.Ready():
				if err != nil {
					lastErr = fmt.Errorf("readiness check failed: %w", err)
					r.service.Stop(proc)
					continue
				}
				return proc, nil
			case <-ctx.Done():
				r.service.Stop(proc)
				return nil, ctx.Err()
			}
		} else {
			return proc, nil
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", config.RetryAttempt, lastErr)
}

func (r *runner) startAllTiers(ctx context.Context, tiers []Tier, processes *[]Process, processesMu *sync.Mutex, wg *sync.WaitGroup) error {
	for tierIdx, tier := range tiers {
		if len(tier.Services) > 0 {
			r.log.Info().Msgf("Starting tier '%s' (%d/%d) with services: %v", tier.Name, tierIdx+1, len(tiers), tier.Services)
		}

		tierProcs, err := r.startTier(ctx, tier.Services, wg)
		processesMu.Lock()
		*processes = append(*processes, tierProcs...)
		processesMu.Unlock()

		if err != nil {
			return err
		}

		if len(tier.Services) > 0 {
			r.log.Info().Msgf("Tier '%s' started successfully, all services ready", tier.Name)
		}
	}

	return nil
}

func (r *runner) shutdown(processes []Process, processesMu *sync.Mutex, wg *sync.WaitGroup) {
	processesMu.Lock()
	r.stopAllProcesses(processes)
	processesMu.Unlock()
	wg.Wait()
}

func (r *runner) stopAllProcesses(processes []Process) {
	for i := range processes {
		idx := len(processes) - 1 - i
		r.service.Stop(processes[idx])
	}
}
