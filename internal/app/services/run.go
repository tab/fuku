package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/fx"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// ProfileResolver expands a profile into ordered service tiers
type ProfileResolver interface {
	Resolve(profile string) ([]model.Tier, error)
}

// Preflight kills the processes still running in the service directories before a launch
type Preflight interface {
	Cleanup(ctx context.Context, dirs map[string]string) error
}

// Launcher starts a service command as a tracked child process
type Launcher interface {
	Start(svc model.Service) (contracts.Process, error)
}

// Tracker is the set of live children the runtime drives
type Tracker interface {
	Get(id string) (contracts.Process, bool)
	Detach(id string)
	Untrack(id string, proc contracts.Process) bool
	Reverse() []contracts.Process
}

// Readiness probes a service address before launch and checks the started process for readiness
type Readiness interface {
	ProbePort(readiness model.Readiness) model.Port
	Check(ctx context.Context, readiness model.Readiness, proc contracts.Process) error
}

// Pool bounds how many service actions run at once
type Pool interface {
	Acquire(ctx context.Context) error
	Release()
}

// Logger is the logging surface the services write through
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// RuntimeParams contains the dependencies of the runtime
type RuntimeParams struct {
	fx.In

	Options    Options
	Profiles   ProfileResolver
	Preflight  Preflight
	Launcher   Launcher
	Tracker    Tracker
	Readiness  Readiness
	Pool       Pool
	Guard      *Guard
	Publisher  contracts.Publisher
	Subscriber contracts.Subscriber
	Reporter   Reporter
	Logger     Logger
}

// Runtime runs a profile: tier startup, the command handler, retries and shutdown
type Runtime struct {
	options    Options
	profiles   ProfileResolver
	preflight  Preflight
	launcher   Launcher
	tracker    Tracker
	readiness  Readiness
	pool       Pool
	guard      *Guard
	publisher  contracts.Publisher
	subscriber contracts.Subscriber
	reporter   Reporter
	loop       *contracts.Loop
	mu         sync.Mutex
	//nolint:containedctx // the command handler runs actions on the work context Run owns
	work   context.Context
	cancel context.CancelCauseFunc
	wg     sync.WaitGroup
	halt   context.CancelFunc
	done   chan struct{}
	log    Logger
}

// NewRuntime creates the services runtime
func NewRuntime(p RuntimeParams) *Runtime {
	return &Runtime{
		options:    p.Options,
		profiles:   p.Profiles,
		preflight:  p.Preflight,
		launcher:   p.Launcher,
		tracker:    p.Tracker,
		readiness:  p.Readiness,
		pool:       p.Pool,
		guard:      p.Guard,
		publisher:  p.Publisher,
		subscriber: p.Subscriber,
		reporter:   p.Reporter,
		done:       make(chan struct{}),
		log:        p.Logger,
	}
}

// Start opens the run before it returns, so StopAll is accepted at once, and runs the profile on its own goroutine
func (r *Runtime) Start(ctx context.Context) error {
	ctx, r.halt = context.WithCancel(ctx)
	work := r.begin(ctx)

	go func() {
		defer close(r.done)

		err := r.run(work)
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, errStopAll) {
			return
		}

		r.reporter.Fail(fmt.Errorf("failed to run profile '%s': %w", r.options.Profile, err))
	}()

	return nil
}

// Stop cancels the run under the launch lock, so no child starts once it began, and waits for every service to stop
func (r *Runtime) Stop(ctx context.Context) error {
	r.mu.Lock()
	r.halt()
	r.mu.Unlock()

	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Done returns a channel that closes once the started run has returned
func (r *Runtime) Done() <-chan struct{} {
	return r.done
}

// run starts the profile tier by tier, serves commands until the run is cancelled and stops every service
func (r *Runtime) run(ctx context.Context) error {
	profile := r.options.Profile
	startupStart := time.Now()

	r.publishPhase(contracts.PhaseChanged{Phase: model.PhaseStartup})

	resolveStart := time.Now()

	tiers, err := r.profiles.Resolve(profile)
	if err != nil {
		r.finish()

		return fmt.Errorf("failed to resolve profile: %w", err)
	}

	r.guard.resolve(tiers)
	r.publish(contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{
			Profile:  profile,
			Tiers:    tiers,
			Duration: time.Since(resolveStart),
		},
	})

	services := profileServices(tiers)
	names := serviceNames(services)

	if len(names) == 0 {
		r.log.Warn(fmt.Sprintf("No services found for profile '%s'. Nothing to run.", profile))
		r.finish()

		return nil
	}

	if err := r.preflight.Cleanup(ctx, serviceDirs(services)); err != nil {
		r.log.Warn("Preflight cleanup failed, continuing startup", "error", err)
	}

	r.log.Info(fmt.Sprintf("Starting services in profile '%s': %v", profile, names))

	r.startAllTiers(ctx, tiers)

	if ctx.Err() != nil {
		r.finish()

		return fmt.Errorf("%w: %w", errStartupInterrupted, context.Cause(ctx))
	}

	r.log.Info("Startup phase complete, waiting for signals...")
	r.guard.setPhase(model.PhaseRunning)
	r.publishPhase(contracts.PhaseChanged{Phase: model.PhaseRunning, Duration: time.Since(startupStart), ServiceCount: len(names)})

	<-ctx.Done()

	r.finish()

	return nil
}

// finish ends the run: the stopping phase, the shutdown of every child, then the stopped phase with duration and count
func (r *Runtime) finish() {
	r.guard.setPhase(model.PhaseStopping)
	r.publishPhase(contracts.PhaseChanged{Phase: model.PhaseStopping})

	start := time.Now()
	count := r.shutdown()
	r.log.Info("All services stopped")

	r.publishPhase(contracts.PhaseChanged{Phase: model.PhaseStopped, Duration: time.Since(start), ServiceCount: count})
	r.guard.setPhase(model.PhaseStopped)
}

// publishPhase announces a phase transition
func (r *Runtime) publishPhase(data contracts.PhaseChanged) {
	r.publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: data})
}

// begin opens the run: the work context the actions run on and the guard, which accepts a StopAll from now on
func (r *Runtime) begin(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancelCause(ctx)

	r.mu.Lock()
	r.work = ctx
	r.cancel = cancel
	r.mu.Unlock()

	r.guard.open()

	return ctx
}

// startAllTiers starts the tiers in order, each waiting for the previous tier, until the run is cancelled
func (r *Runtime) startAllTiers(ctx context.Context, tiers []model.Tier) {
	for i, tier := range tiers {
		if len(tier.Services) == 0 || ctx.Err() != nil {
			continue
		}

		names := serviceNames(tier.Services)
		r.log.Info(fmt.Sprintf("Starting tier '%s' (%d/%d) with services: %v", tier.Name, i+1, len(tiers), names))
		r.publish(contracts.Message{
			Type: contracts.EventTierStarting,
			Data: contracts.TierStarting{Name: tier.Name},
		})

		tierStart := time.Now()
		failed := r.startTier(ctx, tier.Services)

		if len(failed) > 0 {
			r.log.Warn(fmt.Sprintf("Tier '%s' partially failed: %d/%d services failed: %v", tier.Name, len(failed), len(tier.Services), failed))

			continue
		}

		r.log.Info(fmt.Sprintf("Tier '%s' started successfully, all services ready", tier.Name))
		r.publish(contracts.Message{
			Type: contracts.EventTierReady,
			Data: contracts.TierReady{
				Name:         tier.Name,
				Duration:     time.Since(tierStart),
				ServiceCount: len(tier.Services),
			},
		})
	}
}

// startTier starts every service of a tier concurrently within the worker bound and returns the names that failed
func (r *Runtime) startTier(ctx context.Context, services []*model.Service) []string {
	failedChan := make(chan string, len(services))

	var wg sync.WaitGroup

	for _, svc := range services {
		r.guard.dispatch(svc.ID)

		wg.Go(func() {
			defer r.guard.release(svc.ID)

			err := r.pool.Acquire(ctx)
			if err != nil && ctx.Err() != nil {
				r.publishStopped(*svc)

				failedChan <- svc.Name

				return
			}

			if err != nil {
				r.log.Error(fmt.Sprintf("Failed to acquire worker for service '%s'", svc.Name), "error", err)
				r.publish(contracts.Message{
					Type: contracts.EventServiceFailed,
					Data: contracts.ServiceFailed{
						ServiceEvent: contracts.ServiceEvent{Service: *svc, Tier: svc.Tier},
						Error:        fmt.Errorf("%w: %w", contracts.ErrFailedToAcquireWorker, err),
					},
				})

				failedChan <- svc.Name

				return
			}

			defer r.pool.Release()

			if err := r.startWithRetry(ctx, *svc); err != nil {
				failedChan <- svc.Name
			}
		})
	}

	wg.Wait()
	close(failedChan)

	failed := make([]string, 0, len(services))
	for name := range failedChan {
		failed = append(failed, name)
	}

	return failed
}

// profileServices lists the services of every tier in startup order
func profileServices(tiers []model.Tier) []*model.Service {
	var services []*model.Service

	for _, tier := range tiers {
		services = append(services, tier.Services...)
	}

	return services
}

// serviceNames lists the names of the services
func serviceNames(services []*model.Service) []string {
	names := make([]string, len(services))
	for i, svc := range services {
		names[i] = svc.Name
	}

	return names
}
