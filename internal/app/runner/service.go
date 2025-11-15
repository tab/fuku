package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"fuku/internal/app/errors"
	"fuku/internal/app/runtime"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Service handles starting and stopping individual services
type Service interface {
	Start(ctx context.Context, name string, service *config.Service) (Process, error)
	Stop(proc Process) error
}

type service struct {
	readiness Readiness
	log       logger.Logger
	event     runtime.EventBus
}

// NewService creates a new service instance
func NewService(readiness Readiness, log logger.Logger, event runtime.EventBus) Service {
	return &service{
		readiness: readiness,
		log:       log,
		event:     event,
	}
}

// Start starts a service and returns a Process instance
func (s *service) Start(ctx context.Context, name string, svc *config.Service) (Process, error) {
	serviceDir := svc.Dir

	if !filepath.IsAbs(serviceDir) {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errors.ErrFailedToGetWorkingDir, err)
		}

		serviceDir = filepath.Join(wd, serviceDir)
	}

	if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", errors.ErrServiceDirectoryNotExist, serviceDir)
	}

	envFile := filepath.Join(serviceDir, ".env.development")
	if _, err := os.Stat(envFile); err != nil {
		s.log.Warn().Msgf("Environment file not found for service '%s': %s", name, envFile)
	}

	cmd := exec.CommandContext(ctx, "make", "run")
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

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrFailedToStartCommand, err)
	}

	s.log.Info().Msgf("Started service '%s' (PID: %d) in directory: %s", name, cmd.Process.Pid, serviceDir)

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	proc := &process{
		name:         name,
		cmd:          cmd,
		done:         make(chan struct{}),
		ready:        make(chan error, 1),
		stdoutReader: stdoutReader,
		stderrReader: stderrReader,
	}

	go s.teeStream(stdoutPipe, stdoutWriter, name, svc.Tier, "STDOUT")
	go s.teeStream(stderrPipe, stderrWriter, name, svc.Tier, "STDERR")

	go func() {
		defer close(proc.done)

		if err := cmd.Wait(); err != nil {
			s.log.Error().Err(err).Msgf("Service '%s' exited with error", name)
		}

		stdoutWriter.Close()
		stderrWriter.Close()
	}()

	s.handleReadinessCheck(ctx, name, svc, proc, stdoutReader, stderrReader)

	return proc, nil
}

// Stop stops a running service process
func (s *service) Stop(proc Process) error {
	cmd := proc.Cmd()
	if cmd.Process == nil {
		return nil
	}

	s.log.Info().Msgf("Stopping service '%s' (PID: %d)", proc.Name(), cmd.Process.Pid)

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		s.log.Error().Err(err).Msgf("Failed to send SIGTERM to service '%s'", proc.Name())

		if killErr := cmd.Process.Kill(); killErr != nil {
			s.log.Error().Err(killErr).Msgf("Failed to kill service '%s'", proc.Name())
			return killErr
		}

		return err
	}

	select {
	case <-proc.Done():
		return nil
	case <-time.After(config.ShutdownTimeout):
		s.log.Warn().Msgf("Service '%s' did not stop gracefully, forcing kill", proc.Name())

		if killErr := cmd.Process.Kill(); killErr != nil {
			s.log.Error().Err(killErr).Msgf("Failed to kill service '%s'", proc.Name())
			return killErr
		}

		<-proc.Done()

		return nil
	}
}

func (s *service) teeStream(src io.Reader, dst *io.PipeWriter, serviceName, tier, streamType string) {
	scanner := bufio.NewScanner(src)

	if tier == "" {
		tier = config.Default
	}

	for scanner.Scan() {
		line := scanner.Text()
		s.event.Publish(runtime.Event{
			Type: runtime.EventLogLine,
			Data: runtime.LogLineData{
				Service: serviceName,
				Tier:    tier,
				Stream:  streamType,
				Message: line,
			},
		})
		s.log.Info().
			Str("service", serviceName).
			Str("stream", streamType).
			Msg(line)
		fmt.Fprintln(dst, line)
	}

	if err := scanner.Err(); err != nil {
		s.log.Error().Err(err).Msgf("Error reading %s stream for service '%s'", streamType, serviceName)
	}
}

func (s *service) startDraining(stdout, stderr *io.PipeReader) {
	go s.drainPipe(stdout)
	go s.drainPipe(stderr)
}

func (s *service) drainPipe(reader *io.PipeReader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
	}

	if err := scanner.Err(); err != nil {
		s.log.Error().Err(err).Msg("Error draining pipe")
	}
}

func (s *service) handleReadinessCheck(ctx context.Context, name string, svc *config.Service, proc *process, stdout, stderr *io.PipeReader) {
	if svc.Readiness == nil {
		proc.SignalReady(nil)
		s.startDraining(stdout, stderr)

		return
	}

	switch svc.Readiness.Type {
	case config.TypeHTTP:
		s.startDraining(stdout, stderr)

		go s.readiness.Check(ctx, name, svc, proc)
	case config.TypeLog:
		go func() {
			s.readiness.Check(ctx, name, svc, proc)
			s.startDraining(stdout, stderr)
		}()
	default:
		err := fmt.Errorf("unknown readiness type '%s' for service '%s'", svc.Readiness.Type, name)
		s.log.Error().Err(err).Msg("Failed to handle readiness check")
		proc.SignalReady(err)
		s.startDraining(stdout, stderr)
	}
}
