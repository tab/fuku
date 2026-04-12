package runner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/fx"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/app/lifecycle"
	"fuku/internal/app/process"
	"fuku/internal/app/readiness"
	"fuku/internal/app/registry"
	"fuku/internal/app/relay"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

const (
	streamBufferSize = 64 * 1024
	maxLineSize      = 4 * 1024 * 1024

	schemeHTTP  = "http"
	schemeHTTPS = "https"
	portHTTP    = "80"
	portHTTPS   = "443"
)

// Service handles individual service lifecycle
type Service interface {
	Start(ctx context.Context, tier string, svc bus.Service) error
	Stop(id string)
	Restart(ctx context.Context, svc bus.Service)
	Resume(ctx context.Context, svc bus.Service)
}

// ServiceParams contains dependencies for creating a Service
type ServiceParams struct {
	fx.In

	Config      *config.Config
	Lifecycle   lifecycle.Lifecycle
	Readiness   readiness.Readiness
	Registry    registry.Registry
	Guard       Guard
	Bus         bus.Bus
	Broadcaster relay.Broadcaster
	Logger      logger.Logger
}

type service struct {
	cfg         *config.Config
	lifecycle   lifecycle.Lifecycle
	readiness   readiness.Readiness
	registry    registry.Registry
	guard       Guard
	bus         bus.Bus
	broadcaster relay.Broadcaster
	log         logger.Logger
}

// NewService creates a new service instance
func NewService(p ServiceParams) Service {
	return &service{
		cfg:         p.Config,
		lifecycle:   p.Lifecycle,
		readiness:   p.Readiness,
		registry:    p.Registry,
		guard:       p.Guard,
		bus:         p.Bus,
		broadcaster: p.Broadcaster,
		log:         p.Logger.WithComponent("SERVICE"),
	}
}

// Start starts a service with retry logic
func (s *service) Start(ctx context.Context, tier string, svc bus.Service) error {
	cfg := s.cfg.Services[svc.Name]

	var lastErr error

	for attempt := 1; attempt <= s.cfg.Retry.Attempts; attempt++ {
		if attempt > 1 {
			s.log.Info().Msgf("Retrying service '%s' (attempt %d/%d)", svc.Name, attempt, s.cfg.Retry.Attempts)

			select {
			case <-time.After(s.cfg.Retry.Backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		proc, err := s.doStart(ctx, tier, svc, cfg)
		if err != nil {
			lastErr = err

			continue
		}

		s.registry.Add(tier, svc, proc)
		s.watchForExit(svc.ID, proc)

		return nil
	}

	err := fmt.Errorf("%w after %d attempts: %w", errors.ErrMaxRetriesExceeded, s.cfg.Retry.Attempts, lastErr)
	s.log.Error().Err(err).Msgf("Failed to start service '%s'", svc.Name)
	s.bus.Publish(bus.Message{
		Type: bus.EventServiceFailed,
		Data: bus.ServiceFailed{
			ServiceEvent: bus.ServiceEvent{Service: svc, Tier: tier},
			Error:        err,
		},
		Critical: true,
	})

	return err
}

// Stop stops a running service
func (s *service) Stop(id string) {
	lookup := s.registry.Get(id)
	if !lookup.Exists {
		return
	}

	svc := bus.Service{ID: id, Name: lookup.Name}

	s.log.Info().Msgf("Stopping service '%s'", lookup.Name)
	s.bus.Publish(bus.Message{
		Type: bus.EventServiceStopping,
		Data: bus.ServiceStopping{
			ServiceEvent: bus.ServiceEvent{Service: svc, Tier: lookup.Tier},
		},
		Critical: true,
	})

	s.doStop(id, lookup.Proc)

	s.log.Info().Msgf("Service '%s' stopped", lookup.Name)
	s.bus.Publish(bus.Message{
		Type: bus.EventServiceStopped,
		Data: bus.ServiceStopped{
			ServiceEvent: bus.ServiceEvent{Service: svc, Tier: lookup.Tier},
		},
		Critical: true,
	})
}

// Restart restarts a service with guard protection
func (s *service) Restart(ctx context.Context, svc bus.Service) {
	if !s.guard.Lock(svc.ID) {
		s.log.Info().Msgf("Service '%s' restart already in progress, skipping", svc.Name)

		return
	}
	defer s.guard.Unlock(svc.ID)

	cfg, tier := s.getConfig(svc.Name)
	if cfg == nil {
		s.log.Error().Msgf("Service configuration for '%s' not found", svc.Name)
		s.bus.Publish(bus.Message{
			Type: bus.EventServiceFailed,
			Data: bus.ServiceFailed{
				ServiceEvent: bus.ServiceEvent{Service: svc, Tier: ""},
				Error:        errors.ErrServiceNotFound,
			},
			Critical: true,
		})

		return
	}

	s.log.Info().Msgf("Restarting service '%s'", svc.Name)

	s.bus.Publish(bus.Message{
		Type: bus.EventServiceRestarting,
		Data: bus.ServiceRestarting{
			ServiceEvent: bus.ServiceEvent{Service: svc, Tier: tier},
		},
		Critical: true,
	})

	if lookup := s.registry.Get(svc.ID); lookup.Exists {
		s.log.Info().Msgf("Stopping service '%s' before restart", svc.Name)
		s.doStop(svc.ID, lookup.Proc)
	}

	proc, err := s.doStart(ctx, tier, svc, cfg)
	if err != nil {
		s.log.Error().Err(err).Msgf("Failed to restart service '%s'", svc.Name)
		s.bus.Publish(bus.Message{
			Type: bus.EventServiceFailed,
			Data: bus.ServiceFailed{
				ServiceEvent: bus.ServiceEvent{Service: svc, Tier: tier},
				Error:        err,
			},
			Critical: true,
		})

		return
	}

	s.registry.Add(tier, svc, proc)
	s.watchForExit(svc.ID, proc)
}

// Resume starts a stopped or failed service with guard protection
func (s *service) Resume(ctx context.Context, svc bus.Service) {
	if !s.guard.Lock(svc.ID) {
		s.log.Info().Msgf("Service '%s' start already in progress, skipping", svc.Name)

		return
	}
	defer s.guard.Unlock(svc.ID)

	if lookup := s.registry.Get(svc.ID); lookup.Exists {
		s.log.Info().Msgf("Service '%s' is already registered, skipping start", svc.Name)

		return
	}

	cfg, tier := s.getConfig(svc.Name)
	if cfg == nil {
		s.log.Error().Msgf("Service configuration for '%s' not found", svc.Name)
		s.bus.Publish(bus.Message{
			Type: bus.EventServiceFailed,
			Data: bus.ServiceFailed{
				ServiceEvent: bus.ServiceEvent{Service: svc, Tier: ""},
				Error:        errors.ErrServiceNotFound,
			},
			Critical: true,
		})

		return
	}

	//nolint:errcheck // errors published to bus internally by Start
	s.Start(ctx, tier, svc)
}

// doStart creates, starts, and waits for a service to be ready
func (s *service) doStart(ctx context.Context, tier string, svc bus.Service, cfg *config.Service) (process.Process, error) {
	startTime := time.Now()

	serviceDir, envFile, err := s.resolvePaths(svc.Name, cfg.Dir)
	if err != nil {
		return nil, err
	}

	if err := s.preFlightCheck(svc.Name, cfg.Readiness); err != nil {
		return nil, err
	}

	cmd := buildCommand(cfg.Command)
	cmd.Dir = serviceDir

	cmd.Env = append(os.Environ(), "ENV_FILE="+envFile)

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

	s.log.Info().Msgf("Started service '%s' (PID: %d) in directory: %s", svc.Name, cmd.Process.Pid, serviceDir)
	s.bus.Publish(bus.Message{
		Type: bus.EventServiceStarting,
		Data: bus.ServiceStarting{
			ServiceEvent: bus.ServiceEvent{Service: svc, Tier: tier},
			PID:          cmd.Process.Pid,
			Attempt:      1,
		},
		Critical: true,
	})

	proc := s.setupStreams(svc.Name, cmd, stdoutPipe, stderrPipe)
	s.setupReadinessCheck(ctx, svc, cfg, proc)

	if err := s.waitForReady(ctx, proc, cfg); err != nil {
		_ = s.lifecycle.Terminate(proc, config.ShutdownTimeout)

		return nil, err
	}

	s.bus.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: svc, Tier: tier},
			Duration:     time.Since(startTime),
		},
		Critical: true,
	})

	return proc, nil
}

// doStop terminates a running process
func (s *service) doStop(id string, proc process.Process) {
	s.registry.Detach(id)

	_ = s.lifecycle.Terminate(proc, config.ShutdownTimeout)
	<-proc.Done()

	s.registry.Remove(id, proc)
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
func (s *service) setupReadinessCheck(ctx context.Context, svc bus.Service, cfg *config.Service, proc process.Process) {
	stdout := proc.StdoutReader()
	stderr := proc.StderrReader()

	if cfg.Readiness == nil {
		proc.SignalReady(nil)

		go drainPipe(stdout)
		go drainPipe(stderr)

		return
	}

	switch cfg.Readiness.Type {
	case config.TypeHTTP, config.TypeTCP:
		go drainPipe(stdout)
		go drainPipe(stderr)

		go s.readiness.Check(ctx, svc, cfg, proc)
	case config.TypeLog:
		go func() {
			s.readiness.Check(ctx, svc, cfg, proc)

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
func (s *service) watchForExit(id string, proc process.Process) {
	go func() {
		<-proc.Done()

		result := s.registry.Remove(id, proc)
		if !result.Removed || !result.UnexpectedExit {
			return
		}

		svc := bus.Service{ID: id, Name: result.Name}

		s.log.Info().Msgf("Service '%s' exited unexpectedly", result.Name)

		if s.isWatched(result.Name) {
			s.bus.Publish(bus.Message{
				Type: bus.EventServiceFailed,
				Data: bus.ServiceFailed{
					ServiceEvent: bus.ServiceEvent{Service: svc, Tier: result.Tier},
					Error:        errors.ErrUnexpectedExit,
				},
				Critical: true,
			})
		} else {
			s.bus.Publish(bus.Message{
				Type: bus.EventServiceStopped,
				Data: bus.ServiceStopped{
					ServiceEvent: bus.ServiceEvent{Service: svc, Tier: result.Tier},
					Unexpected:   true,
				},
				Critical: true,
			})
		}
	}()
}

// teeStream reads from source and writes to destination while logging
func (s *service) teeStream(src io.Reader, dst *io.PipeWriter, serviceName, streamType string) {
	isEnabled := s.shouldLogStream(serviceName, streamType)
	if !isEnabled {
		//nolint:errcheck // pipe write errors are handled by the reader
		io.Copy(dst, src)

		return
	}

	reader := bufio.NewReaderSize(src, streamBufferSize)

	var buf bytes.Buffer

	for {
		line, isPrefix, err := reader.ReadLine()
		if len(line) > 0 {
			//nolint:errcheck // pipe write errors are handled by the reader
			dst.Write(line)
		}

		if len(line) > 0 && buf.Len()+len(line) <= maxLineSize {
			buf.Write(line)
		}

		if !isPrefix && (len(line) > 0 || buf.Len() > 0 || err == nil) {
			//nolint:errcheck // pipe write errors are handled by the reader
			dst.Write([]byte{'\n'})

			text := buf.String()
			s.log.Info().Str("service", serviceName).Str("stream", streamType).Msg(text)

			if s.broadcaster != nil {
				s.broadcaster.Broadcast(serviceName, text)
			}

			buf.Reset()
		}

		if err != nil && err != io.EOF {
			s.log.Error().Err(err).Msgf("Error reading %s stream for service '%s'", streamType, serviceName)
		}

		if err != nil {
			break
		}
	}
}

// shouldLogStream returns whether a service stream should be logged to console
func (s *service) shouldLogStream(name, streamType string) bool {
	cfg, exists := s.cfg.Services[name]
	if !exists {
		return false
	}

	if cfg.Logs == nil || len(cfg.Logs.Output) == 0 {
		return true
	}

	for _, output := range cfg.Logs.Output {
		if strings.EqualFold(output, streamType) {
			return true
		}
	}

	return false
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

// preFlightCheck verifies the service port is not already in use
func (s *service) preFlightCheck(name string, r *config.Readiness) error {
	address := s.extractAddress(r)
	if address == "" {
		return nil
	}

	conn, err := net.DialTimeout("tcp", address, config.PreFlightTimeout)
	if err != nil {
		//nolint:nilerr // dial failure means port is free, not an error
		return nil
	}

	conn.Close()
	s.log.Warn().Msgf("Service '%s' address %s is already in use", name, address)

	return fmt.Errorf("%w: %s", errors.ErrPortAlreadyInUse, address)
}

// extractAddress returns the host:port from readiness configuration
func (s *service) extractAddress(r *config.Readiness) string {
	if r == nil {
		return ""
	}

	switch r.Type {
	case config.TypeHTTP:
		return extractFromURL(r.URL)
	case config.TypeTCP:
		return r.Address
	default:
		return ""
	}
}

// extractFromURL extracts host:port from URL (e.g., "http://localhost:8080/health" -> "localhost:8080")
func extractFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	host := parsed.Hostname()
	if host == "" {
		return ""
	}

	port := parsed.Port()
	if port == "" {
		switch parsed.Scheme {
		case schemeHTTP:
			port = portHTTP
		case schemeHTTPS:
			port = portHTTPS
		default:
			return ""
		}
	}

	return net.JoinHostPort(host, port)
}

// isWatched returns true if the service has watch configuration
func (s *service) isWatched(name string) bool {
	cfg, exists := s.cfg.Services[name]

	return exists && cfg.Watch != nil
}

// buildCommand creates an exec.Cmd from a command string, defaulting to "make run"
func buildCommand(command string) *exec.Cmd {
	if command == "" {
		return exec.Command("make", "run")
	}

	return exec.Command("sh", "-c", command)
}

func drainPipe(reader io.Reader) {
	//nolint:errcheck // intentionally draining pipe
	io.Copy(io.Discard, reader)
}
