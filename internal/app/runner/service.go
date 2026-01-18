package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"fuku/internal/app/errors"
	"fuku/internal/app/runtime"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

const (
	// scannerBufferSize is the initial buffer size for reading service output (64KB)
	scannerBufferSize = 64 * 1024
	// scannerBufferSize is the maximum buffer size for reading service output (4MB)
	scannerMaxBufferSize = 4 * 1024 * 1024
)

// Service handles starting and stopping individual services
type Service interface {
	Start(ctx context.Context, name string, service *config.Service) (Process, error)
	Stop(proc Process) error
}

type service struct {
	lifecycle Lifecycle
	readiness Readiness
	event     runtime.EventBus
	log       logger.Logger
}

// NewService creates a new service instance
func NewService(lifecycle Lifecycle, readiness Readiness, event runtime.EventBus, log logger.Logger) Service {
	return &service{
		lifecycle: lifecycle,
		readiness: readiness,
		event:     event,
		log:       log,
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

	s.lifecycle.Configure(cmd)

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

	go s.teeStream(stdoutPipe, stdoutWriter, name, "STDOUT")
	go s.teeStream(stderrPipe, stderrWriter, name, "STDERR")

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
	return s.lifecycle.Terminate(proc, config.ShutdownTimeout)
}

func (s *service) teeStream(src io.Reader, dst *io.PipeWriter, serviceName, streamType string) {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, scannerBufferSize), scannerMaxBufferSize)

	for scanner.Scan() {
		line := scanner.Text()
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
	scanner.Buffer(make([]byte, scannerBufferSize), scannerMaxBufferSize)

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
