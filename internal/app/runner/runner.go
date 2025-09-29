//go:generate mockgen -source=runner.go -destination=runner_mock.go -package=runner
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
	"strings"
	"sync"
	"syscall"
	"time"

	"fuku/internal/app/colors"
	"fuku/internal/app/tracker"
	"fuku/internal/app/ui"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Runner defines the interface for service orchestration and execution
type Runner interface {
	Run(ctx context.Context, profile string) error
}

type runner struct {
	cfg     *config.Config
	display ui.Display
	log     logger.Logger
}

const (
	scannerInitialBufferSize = 64 * 1024   // 64KB initial buffer
	scannerMaxBufferSize     = 1024 * 1024 // 1MB max buffer
)

type serviceProcess struct {
	name string
	cmd  *exec.Cmd
	done chan struct{}
}

type serviceLayer struct {
	services []string
	level    int
}

// NewRunner creates a new runner instance with the provided configuration and logger
func NewRunner(cfg *config.Config, display ui.Display, log logger.Logger) Runner {
	return &runner{
		cfg:     cfg,
		display: display,
		log:     log,
	}
}

// Run executes the specified profile by starting all services in dependency order
func (r *runner) Run(ctx context.Context, profile string) error {
	serviceNames, err := r.cfg.GetServicesForProfile(profile)
	if err != nil {
		return fmt.Errorf("failed to resolve profile services: %w", err)
	}

	layers, err := r.buildDependencyLayers(serviceNames)
	if err != nil {
		return fmt.Errorf("failed to resolve service dependencies: %w", err)
	}

	r.display.Phase("Discovery")
	fmt.Printf("  %s %s Found %d services in profile '%s'\n\n",
		colors.ProgressArrow,
		colors.StatusSuccessColor(colors.StatusSuccess),
		len(serviceNames),
		colors.Primary(profile))

	r.display.Phase("Planning")
	fmt.Printf("  %s %s Resolved dependencies → %d layers\n\n",
		colors.ProgressArrow,
		colors.StatusSuccessColor(colors.StatusSuccess),
		len(layers))

	r.display.Phase("Execution")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	processes := make(map[string]*serviceProcess)

	for _, serviceName := range serviceNames {
		r.display.Add(serviceName)
		process := &serviceProcess{
			name: serviceName,
		}
		processes[serviceName] = process
	}

	errorChan := make(chan error, len(serviceNames))
	var wg sync.WaitGroup

	go func() {
		if err := r.startServicesInLayers(ctx, layers, processes, &wg); err != nil {
			errorChan <- err
			return
		}

		if r.display.IsReady() && r.display.IsBootstrap() {
			r.display.ShowSummary()
			r.display.ShowSuccess()
		}
	}()

	select {
	case err := <-errorChan:
		r.log.Error().Err(err).Msg("Service startup failed")
		cancel()
		r.stopAllProcesses(processes)
		wg.Wait()
		r.display.ShowError("Multiple services", err)
		return err
	case sig := <-sigChan:
		r.log.Info().Msgf("Received signal %s, shutting down services...", sig)
		cancel()
	case <-ctx.Done():
		r.log.Info().Msg("Context cancelled, shutting down services...")
	}

	r.stopAllProcesses(processes)
	wg.Wait()

	select {
	case <-errorChan:
		r.display.ShowError("Startup", fmt.Errorf("services failed to start properly"))
	default:
		if r.display.IsReady() {
			r.display.ShowSummary()
			r.display.ShowSuccess()
		} else {
			r.display.ShowSummary()
		}
	}

	r.log.Info().Msg("All services stopped")
	return nil
}

func (r *runner) startServicesInLayers(ctx context.Context, layers []serviceLayer, processes map[string]*serviceProcess, wg *sync.WaitGroup) error {
	for _, layer := range layers {
		r.display.SetProgress(layer.level+1, len(layers), fmt.Sprintf("Layer %d: %s", layer.level, strings.Join(layer.services, ", ")))
		r.display.ShowLayer(layer.level, layer.services)

		layerWg := sync.WaitGroup{}
		layerErrorChan := make(chan error, len(layer.services))

		var displayMutex sync.Mutex

		for _, serviceName := range layer.services {
			layerWg.Add(1)
			go func(name string) {
				defer layerWg.Done()

				process := processes[name]
				service := r.cfg.Services[name]

				r.display.Update(name, tracker.StatusStarting)

				displayMutex.Lock()
				if r.display.IsBootstrap() {
					r.display.UpdateLayer(layer.level, layer.services)
					time.Sleep(50 * time.Millisecond)
				}
				displayMutex.Unlock()

				if err := r.startSingleService(ctx, name, service, process, wg); err != nil {
					r.display.Update(name, tracker.StatusFailed)
					r.display.Error(name, err)

					displayMutex.Lock()
					if r.display.IsBootstrap() {
						r.display.UpdateLayer(layer.level, layer.services)
						time.Sleep(50 * time.Millisecond)
					}
					displayMutex.Unlock()

					layerErrorChan <- fmt.Errorf("failed to start service '%s': %w", name, err)
					return
				}

				r.display.Update(name, tracker.StatusRunning)

				displayMutex.Lock()
				if r.display.IsBootstrap() {
					r.display.UpdateLayer(layer.level, layer.services)
					time.Sleep(50 * time.Millisecond)
				}
				displayMutex.Unlock()
			}(serviceName)
		}

		layerWg.Wait()
		close(layerErrorChan)

		if err := <-layerErrorChan; err != nil {
			return err
		}

		if layer.level < len(layers)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return nil
}

func (r *runner) startSingleService(ctx context.Context, name string, service *config.Service, process *serviceProcess, wg *sync.WaitGroup) error {
	serviceDir := service.Dir
	if !filepath.IsAbs(serviceDir) {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		serviceDir = filepath.Join(wd, serviceDir)
	}

	if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
		return fmt.Errorf("service directory does not exist: %s", serviceDir)
	}

	envFile := filepath.Join(serviceDir, ".env.development")
	if _, err := os.Stat(envFile); err != nil {
		if !r.display.IsBootstrap() {
			r.log.Debug().Msgf("Environment file not found for service '%s': %s", name, envFile)
		}
	}

	cmd := exec.CommandContext(ctx, "make", "run")
	cmd.Dir = serviceDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("ENV_FILE=%s", envFile))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	if !r.display.IsBootstrap() {
		r.log.Info().Msgf("Started service '%s' (PID: %d) in directory: %s", name, cmd.Process.Pid, serviceDir)
	}

	process.cmd = cmd
	process.done = make(chan struct{})

	go r.streamLogs(name, stdout, "STDOUT")
	go r.streamLogs(name, stderr, "STDERR")

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(process.done)
		if err := cmd.Wait(); err != nil {
			r.display.Update(name, tracker.StatusFailed)
			r.display.Error(name, err)
			if !r.display.IsBootstrap() {
				r.log.Error().Err(err).Msgf("Service '%s' exited with error", name)
			}
		}
	}()

	return nil
}

func (r *runner) streamLogs(serviceName string, reader io.Reader, streamType string) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, scannerInitialBufferSize), scannerMaxBufferSize)
	for scanner.Scan() {
		line := scanner.Text()
		var logLine string
		if streamType == "STDERR" {
			logLine = fmt.Sprintf("[%s:ERR] %s", serviceName, line)
		} else {
			logLine = fmt.Sprintf("[%s] %s", serviceName, line)
		}

		if r.display.IsBootstrap() {
			r.display.BufferLog(logLine)
		} else {
			fmt.Println(logLine)
		}
	}
	if err := scanner.Err(); err != nil {
		if !r.display.IsBootstrap() {
			r.log.Error().Err(err).Msgf("Error reading %s stream for service '%s'", streamType, serviceName)
		}
	}
}

func (r *runner) stopAllProcesses(processes map[string]*serviceProcess) {
	for _, process := range processes {
		if process.cmd != nil && process.cmd.Process != nil {
			r.log.Info().Msgf("Stopping service '%s' (PID: %d)", process.name, process.cmd.Process.Pid)
			r.display.Update(process.name, tracker.StatusStopped)
			if err := process.cmd.Process.Signal(syscall.SIGTERM); err != nil {
				r.log.Error().Err(err).Msgf("Failed to send SIGTERM to service '%s'", process.name)
				if killErr := process.cmd.Process.Kill(); killErr != nil {
					r.log.Error().Err(killErr).Msgf("Failed to kill service '%s'", process.name)
				}
			}
		}
	}
}

func (r *runner) buildDependencyLayers(serviceNames []string) ([]serviceLayer, error) {
	layers := []serviceLayer{}
	remaining := make(map[string]bool)
	for _, name := range serviceNames {
		remaining[name] = true
	}

	level := 0
	for len(remaining) > 0 {
		currentLayer := []string{}

		for serviceName := range remaining {
			service, exists := r.cfg.Services[serviceName]
			if !exists {
				return nil, fmt.Errorf("service '%s' not found", serviceName)
			}

			canStart := true
			for _, dep := range service.DependsOn {
				if remaining[dep] {
					canStart = false
					break
				}
			}

			if canStart {
				currentLayer = append(currentLayer, serviceName)
			}
		}

		if len(currentLayer) == 0 {
			remainingServices := make([]string, 0, len(remaining))
			for name := range remaining {
				remainingServices = append(remainingServices, name)
			}
			return nil, fmt.Errorf("circular dependency detected among services: %v", remainingServices)
		}

		layers = append(layers, serviceLayer{
			services: currentLayer,
			level:    level,
		})

		for _, serviceName := range currentLayer {
			delete(remaining, serviceName)
		}

		level++
	}

	return layers, nil
}
