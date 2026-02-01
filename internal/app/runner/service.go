package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/app/lifecycle"
	"fuku/internal/app/logs"
	"fuku/internal/app/process"
	"fuku/internal/app/readiness"
	"fuku/internal/app/registry"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

const (
	scannerBufferSize    = 64 * 1024
	scannerMaxBufferSize = 4 * 1024 * 1024
)

// Service handles individual service lifecycle
type Service interface {
	Start(ctx context.Context, name, tier string) error
	Stop(name string)
	Restart(ctx context.Context, name string)
}

type service struct {
	cfg       *config.Config
	lifecycle lifecycle.Lifecycle
	readiness readiness.Readiness
	registry  registry.Registry
	guard     Guard
	bus       bus.Bus
	server    logs.Server
	log       logger.Logger
}

// NewService creates a new service instance
func NewService(
	cfg *config.Config,
	lc lifecycle.Lifecycle,
	rd readiness.Readiness,
	reg registry.Registry,
	guard Guard,
	b bus.Bus,
	server logs.Server,
	log logger.Logger,
) Service {
	return &service{
		cfg:       cfg,
		lifecycle: lc,
		readiness: rd,
		registry:  reg,
		guard:     guard,
		bus:       b,
		server:    server,
		log:       log.WithComponent("SERVICE"),
	}
}

// Start starts a service with retry logic
func (s *service) Start(ctx context.Context, name, tier string) error {
	cfg := s.cfg.Services[name]

	var lastErr error

	for attempt := 1; attempt <= s.cfg.Retry.Attempts; attempt++ {
		if attempt > 1 {
			s.log.Info().Msgf("Retrying service '%s' (attempt %d/%d)", name, attempt, s.cfg.Retry.Attempts)

			select {
			case <-time.After(s.cfg.Retry.Backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		proc, err := s.doStart(ctx, name, tier, cfg)
		if err != nil {
			lastErr = err

			continue
		}

		s.registry.Add(name, proc, tier)
		s.watchForExit(proc)

		return nil
	}

	err := fmt.Errorf("%w after %d attempts: %w", errors.ErrMaxRetriesExceeded, s.cfg.Retry.Attempts, lastErr)
	s.log.Error().Err(err).Msgf("Failed to start service '%s'", name)
	s.bus.Publish(bus.Message{
		Type: bus.EventServiceFailed,
		Data: bus.ServiceFailed{
			ServiceEvent: bus.ServiceEvent{Service: name, Tier: tier},
			Error:        err,
		},
		Critical: true,
	})

	return err
}

// Stop stops a running service
func (s *service) Stop(name string) {
	lookup := s.registry.Get(name)
	if !lookup.Exists {
		return
	}

	s.log.Info().Msgf("Stopping service '%s'", name)
	s.bus.Publish(bus.Message{
		Type: bus.EventServiceStopping,
		Data: bus.ServiceStopping{
			ServiceEvent: bus.ServiceEvent{Service: name, Tier: lookup.Tier},
		},
		Critical: true,
	})

	s.doStop(name, lookup.Proc)

	s.log.Info().Msgf("Service '%s' stopped", name)
	s.bus.Publish(bus.Message{
		Type: bus.EventServiceStopped,
		Data: bus.ServiceStopped{
			ServiceEvent: bus.ServiceEvent{Service: name, Tier: lookup.Tier},
		},
		Critical: true,
	})
}

// Restart restarts a service with guard protection
func (s *service) Restart(ctx context.Context, name string) {
	if !s.guard.Lock(name) {
		s.log.Info().Msgf("Service '%s' restart already in progress, skipping", name)

		return
	}
	defer s.guard.Unlock(name)

	cfg, tier := s.getConfig(name)
	if cfg == nil {
		s.log.Error().Msgf("Service configuration for '%s' not found", name)
		s.bus.Publish(bus.Message{
			Type: bus.EventServiceFailed,
			Data: bus.ServiceFailed{
				ServiceEvent: bus.ServiceEvent{Service: name, Tier: ""},
				Error:        errors.ErrServiceNotFound,
			},
			Critical: true,
		})

		return
	}

	s.log.Info().Msgf("Restarting service '%s'", name)
	s.bus.Publish(bus.Message{
		Type: bus.EventServiceRestarting,
		Data: bus.ServiceRestarting{
			ServiceEvent: bus.ServiceEvent{Service: name, Tier: tier},
		},
		Critical: true,
	})

	if lookup := s.registry.Get(name); lookup.Exists {
		s.log.Info().Msgf("Stopping service '%s' before restart", name)
		s.doStop(name, lookup.Proc)
	}

	proc, err := s.doStart(ctx, name, tier, cfg)
	if err != nil {
		s.log.Error().Err(err).Msgf("Failed to restart service '%s'", name)
		s.bus.Publish(bus.Message{
			Type: bus.EventServiceFailed,
			Data: bus.ServiceFailed{
				ServiceEvent: bus.ServiceEvent{Service: name, Tier: tier},
				Error:        err,
			},
			Critical: true,
		})

		return
	}

	s.registry.Add(name, proc, tier)
	s.watchForExit(proc)
}

// doStart creates, starts, and waits for a service to be ready
func (s *service) doStart(ctx context.Context, name, tier string, cfg *config.Service) (process.Process, error) {
	startTime := time.Now()

	serviceDir, envFile, err := s.resolvePaths(name, cfg.Dir)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("make", "run")
	cmd.Dir = serviceDir

	cmd.Env = append(os.Environ(), fmt.Sprintf("ENV_FILE=%s", envFile))

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%w (stdout): %w", errors.ErrFailedToCreatePipe, err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("%w (stderr): %w", errors.ErrFailedToCreatePipe, err)
	}

	s.lifecycle.Configure(cmd)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrFailedToStartCommand, err)
	}

	s.log.Info().Msgf("Started service '%s' (PID: %d) in directory: %s", name, cmd.Process.Pid, serviceDir)
	s.bus.Publish(bus.Message{
		Type: bus.EventServiceStarting,
		Data: bus.ServiceStarting{
			ServiceEvent: bus.ServiceEvent{Service: name, Tier: tier},
			PID:          cmd.Process.Pid,
			Attempt:      1,
		},
		Critical: true,
	})

	proc := s.setupStreams(name, cmd, stdoutPipe, stderrPipe)
	s.setupReadinessCheck(ctx, name, cfg, proc)

	if err := s.waitForReady(ctx, proc, cfg); err != nil {
		_ = s.lifecycle.Terminate(proc, config.ShutdownTimeout)

		return nil, err
	}

	s.bus.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: name, Tier: tier},
			Duration:     time.Since(startTime),
		},
		Critical: true,
	})

	return proc, nil
}

// doStop terminates a running process
func (s *service) doStop(name string, proc process.Process) {
	s.registry.Detach(name)

	_ = s.lifecycle.Terminate(proc, config.ShutdownTimeout)
	<-proc.Done()

	s.registry.Remove(name, proc)
}

// resolvePaths validates and returns absolute paths for service directory and env file
func (s *service) resolvePaths(name, dir string) (serviceDir, envFile string, err error) {
	serviceDir = dir

	if !filepath.IsAbs(serviceDir) {
		wd, err := os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("%w: %w", errors.ErrFailedToGetWorkingDir, err)
		}

		serviceDir = filepath.Join(wd, serviceDir)
	}

	if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
		return "", "", fmt.Errorf("%w: %s", errors.ErrServiceDirectoryNotExist, serviceDir)
	}

	envFile = filepath.Join(serviceDir, ".env.development")
	if _, err := os.Stat(envFile); err != nil {
		s.log.Warn().Msgf("Environment file not found for service '%s': %s", name, envFile)
	}

	return serviceDir, envFile, nil
}

// setupStreams creates process handle and starts stream goroutines
func (s *service) setupStreams(name string, cmd *exec.Cmd, stdoutPipe, stderrPipe io.ReadCloser) process.Process {
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	proc := process.NewProcess(process.Params{
		Name:         name,
		Cmd:          cmd,
		StdoutReader: stdoutReader,
		StderrReader: stderrReader,
	})

	go s.teeStream(stdoutPipe, stdoutWriter, name, "STDOUT")
	go s.teeStream(stderrPipe, stderrWriter, name, "STDERR")

	go func() {
		defer proc.Close()

		if err := cmd.Wait(); err != nil {
			s.log.Error().Err(err).Msgf("Service '%s' exited with error", name)
		}

		stdoutWriter.Close()
		stderrWriter.Close()
	}()

	return proc
}

// setupReadinessCheck configures the appropriate readiness check
func (s *service) setupReadinessCheck(ctx context.Context, name string, cfg *config.Service, proc process.Process) {
	stdout := proc.StdoutReader()
	stderr := proc.StderrReader()

	if cfg.Readiness == nil {
		proc.SignalReady(nil)

		go drainPipe(stdout)
		go drainPipe(stderr)

		return
	}

	switch cfg.Readiness.Type {
	case config.TypeHTTP:
		go drainPipe(stdout)
		go drainPipe(stderr)

		go s.readiness.Check(ctx, name, cfg, proc)
	case config.TypeLog:
		go func() {
			s.readiness.Check(ctx, name, cfg, proc)

			go drainPipe(stdout)
			go drainPipe(stderr)
		}()
	default:
		proc.SignalReady(fmt.Errorf("unknown readiness type '%s'", cfg.Readiness.Type))

		go drainPipe(stdout)
		go drainPipe(stderr)
	}
}

// waitForReady waits for process readiness if configured
func (s *service) waitForReady(ctx context.Context, proc process.Process, cfg *config.Service) error {
	if cfg.Readiness == nil {
		return nil
	}

	select {
	case err := <-proc.Ready():
		if err != nil {
			return fmt.Errorf("readiness check failed: %w", err)
		}

		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// watchForExit monitors process and handles unexpected exits
func (s *service) watchForExit(proc process.Process) {
	go func() {
		<-proc.Done()

		result := s.registry.Remove(proc.Name(), proc)
		if !result.Removed || !result.UnexpectedExit {
			return
		}

		s.log.Info().Msgf("Service '%s' exited unexpectedly", proc.Name())

		if s.isWatched(proc.Name()) {
			s.bus.Publish(bus.Message{
				Type: bus.EventServiceFailed,
				Data: bus.ServiceFailed{
					ServiceEvent: bus.ServiceEvent{Service: proc.Name(), Tier: result.Tier},
					Error:        errors.ErrUnexpectedExit,
				},
				Critical: true,
			})
		} else {
			s.bus.Publish(bus.Message{
				Type: bus.EventServiceStopped,
				Data: bus.ServiceStopped{
					ServiceEvent: bus.ServiceEvent{Service: proc.Name(), Tier: result.Tier},
				},
				Critical: true,
			})
		}
	}()
}

// teeStream reads from source and writes to destination while logging
func (s *service) teeStream(src io.Reader, dst *io.PipeWriter, serviceName, streamType string) {
	scanner := newScanner(src)

	for scanner.Scan() {
		line := scanner.Text()
		s.log.Info().Str("service", serviceName).Str("stream", streamType).Msg(line)
		fmt.Fprintln(dst, line)

		if s.server != nil {
			s.server.Broadcast(serviceName, line)
		}
	}

	if err := scanner.Err(); err != nil {
		s.log.Error().Err(err).Msgf("Error reading %s stream for service '%s'", streamType, serviceName)
	}
}

// getConfig returns service configuration and tier
func (s *service) getConfig(name string) (*config.Service, string) {
	cfg, exists := s.cfg.Services[name]
	if !exists {
		return nil, ""
	}

	tier := cfg.Tier
	if tier == "" {
		tier = config.Default
	}

	return cfg, tier
}

// isWatched returns true if the service has watch configuration
func (s *service) isWatched(name string) bool {
	cfg, exists := s.cfg.Services[name]

	return exists && cfg.Watch != nil
}

func newScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, scannerBufferSize), scannerMaxBufferSize)

	return scanner
}

func drainPipe(reader io.Reader) {
	scanner := newScanner(reader)
	for scanner.Scan() {
	}
}
