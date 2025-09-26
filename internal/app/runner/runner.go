package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Runner defines the interface for service orchestration and execution
type Runner interface {
	// Run executes the specified profile by starting all services in dependency order
	Run(ctx context.Context, profile string) error
}

type runner struct {
	cfg *config.Config
	log logger.Logger
}

type serviceProcess struct {
	name string
	cmd  *exec.Cmd
	done chan struct{}
}

// NewRunner creates a new runner instance with the provided configuration and logger
func NewRunner(cfg *config.Config, log logger.Logger) Runner {
	return &runner{
		cfg: cfg,
		log: log,
	}
}

// Run executes the specified profile by starting all services in dependency order
func (r *runner) Run(ctx context.Context, profile string) error {
	serviceNames, err := r.cfg.GetServicesForProfile(profile)
	if err != nil {
		return fmt.Errorf("failed to resolve profile services: %w", err)
	}

	services, err := r.resolveServiceOrder(serviceNames)
	if err != nil {
		return fmt.Errorf("failed to resolve service dependencies: %w", err)
	}

	r.log.Info().Msgf("Starting services in profile '%s': %v", profile, services)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	processes := make([]*serviceProcess, 0, len(services))
	var wg sync.WaitGroup

	for _, serviceName := range services {
		service, exists := r.cfg.Services[serviceName]
		if !exists {
			return fmt.Errorf("service '%s' not found in configuration", serviceName)
		}

		process, err := r.startService(ctx, serviceName, service)
		if err != nil {
			r.stopAllProcesses(processes)
			return fmt.Errorf("failed to start service '%s': %w", serviceName, err)
		}

		processes = append(processes, process)

		wg.Add(1)
		go func(p *serviceProcess) {
			defer wg.Done()
			<-p.done
			r.log.Info().Msgf("Service '%s' stopped", p.name)
		}(process)

		time.Sleep(2 * time.Second)
	}

	select {
	case sig := <-sigChan:
		r.log.Info().Msgf("Received signal %s, shutting down services...", sig)
		cancel()
	case <-ctx.Done():
		r.log.Info().Msg("Context cancelled, shutting down services...")
	}

	r.stopAllProcesses(processes)
	wg.Wait()

	r.log.Info().Msg("All services stopped")
	return nil
}

func (r *runner) startService(ctx context.Context, name string, service *config.Service) (*serviceProcess, error) {
	serviceDir := service.Dir
	if !filepath.IsAbs(serviceDir) {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		serviceDir = filepath.Join(wd, serviceDir)
	}

	if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("service directory does not exist: %s", serviceDir)
	}

	envFile := filepath.Join(serviceDir, ".env.development")
	if _, err := os.Stat(envFile); err != nil {
		r.log.Warn().Msgf("Environment file not found for service '%s': %s", name, envFile)
	}

	cmd := exec.CommandContext(ctx, "make", "run")
	cmd.Dir = serviceDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("ENV_FILE=%s", envFile))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	r.log.Info().Msgf("Started service '%s' (PID: %d) in directory: %s", name, cmd.Process.Pid, serviceDir)

	process := &serviceProcess{
		name: name,
		cmd:  cmd,
		done: make(chan struct{}),
	}

	go r.streamLogs(name, stdout, "STDOUT")
	go r.streamLogs(name, stderr, "STDERR")

	go func() {
		defer close(process.done)
		if err := cmd.Wait(); err != nil {
			r.log.Error().Err(err).Msgf("Service '%s' exited with error", name)
		}
	}()

	return process, nil
}

func (r *runner) streamLogs(serviceName string, reader io.Reader, streamType string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Printf("[%s:%s] %s\n", serviceName, streamType, line)
	}
	if err := scanner.Err(); err != nil {
		r.log.Error().Err(err).Msgf("Error reading %s stream for service '%s'", streamType, serviceName)
	}
}

func (r *runner) stopAllProcesses(processes []*serviceProcess) {
	for i := len(processes) - 1; i >= 0; i-- {
		process := processes[i]
		if process.cmd.Process != nil {
			r.log.Info().Msgf("Stopping service '%s' (PID: %d)", process.name, process.cmd.Process.Pid)
			if err := process.cmd.Process.Signal(syscall.SIGTERM); err != nil {
				r.log.Error().Err(err).Msgf("Failed to send SIGTERM to service '%s'", process.name)
				if killErr := process.cmd.Process.Kill(); killErr != nil {
					r.log.Error().Err(killErr).Msgf("Failed to kill service '%s'", process.name)
				}
			}
		}
	}
}

func (r *runner) resolveServiceOrder(serviceNames []string) ([]string, error) {
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	result := make([]string, 0, len(serviceNames))

	var visit func(string) error
	visit = func(serviceName string) error {
		if visiting[serviceName] {
			return fmt.Errorf("circular dependency detected for service '%s'", serviceName)
		}
		if visited[serviceName] {
			return nil
		}

		visiting[serviceName] = true

		service, exists := r.cfg.Services[serviceName]
		if !exists {
			return fmt.Errorf("service '%s' not found", serviceName)
		}

		for _, dep := range service.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[serviceName] = false
		visited[serviceName] = true
		result = append(result, serviceName)

		return nil
	}

	for _, serviceName := range serviceNames {
		if err := visit(serviceName); err != nil {
			return nil, err
		}
	}

	return result, nil
}
