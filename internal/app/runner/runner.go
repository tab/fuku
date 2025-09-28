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

	"fuku/internal/app/results"
	"fuku/internal/app/ui"
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

const (
	// Scanner buffer sizes for log streaming
	scannerInitialBufferSize = 64 * 1024   // 64KB initial buffer
	scannerMaxBufferSize     = 1024 * 1024 // 1MB max buffer
)

type serviceProcess struct {
	name   string
	cmd    *exec.Cmd
	done   chan struct{}
	result *results.ServiceResult
}

type serviceLayer struct {
	services []string
	level    int
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

	layers, err := r.buildDependencyLayers(serviceNames)
	if err != nil {
		return fmt.Errorf("failed to resolve service dependencies: %w", err)
	}

	display := ui.NewDisplay()
	display.Phase("1", "🔍 Discovery")
	fmt.Printf("  └─ ✅ Found %d services in profile '%s'\n\n", len(serviceNames), profile)

	display.Phase("2", "🧠 Planning")
	fmt.Printf("  └─ ✅ Resolved dependencies → %d layers\n\n", len(layers))

	display.Phase("3", "🚀 Execution")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	tracker := results.NewResultsTracker()
	processes := make(map[string]*serviceProcess)

	for _, serviceName := range serviceNames {
		result := tracker.AddService(serviceName)
		display.AddService(serviceName)
		process := &serviceProcess{
			name:   serviceName,
			result: result,
		}
		processes[serviceName] = process
	}

	errorChan := make(chan error, len(serviceNames))
	var wg sync.WaitGroup

	go func() {
		if err := r.startServicesInLayers(ctx, layers, processes, display, &wg); err != nil {
			errorChan <- err
			return
		}

		if display.AreAllServicesRunning() && display.IsBootstrapMode() {
			display.DisplayBootstrapSummary()
			display.DisplaySuccess()
		}
	}()

	select {
	case err := <-errorChan:
		r.log.Error().Err(err).Msg("Service startup failed")
		cancel()
		r.stopAllProcesses(processes, display)
		wg.Wait()
		display.DisplayError("Multiple services", err)
		return err
	case sig := <-sigChan:
		r.log.Info().Msgf("Received signal %s, shutting down services...", sig)
		cancel()
	case <-ctx.Done():
		r.log.Info().Msg("Context cancelled, shutting down services...")
	}

	r.stopAllProcesses(processes, display)
	wg.Wait()

	// Check if we should show success or error
	select {
	case <-errorChan:
		display.DisplayError("Startup", fmt.Errorf("services failed to start properly"))
	default:
		// Only show success if all services are actually running
		if display.AreAllServicesRunning() {
			display.DisplayBootstrapSummary()
			display.DisplaySuccess()
		} else {
			display.DisplayBootstrapSummary()
		}
	}

	r.log.Info().Msg("All services stopped")
	return nil
}

func (r *runner) startServicesInLayers(ctx context.Context, layers []serviceLayer, processes map[string]*serviceProcess, display *ui.Display, wg *sync.WaitGroup) error {
	for _, layer := range layers {
		display.UpdateProgress(layer.level+1, len(layers), fmt.Sprintf("Layer %d: %s", layer.level, strings.Join(layer.services, ", ")))
		display.DisplayLayerProgress(layer.level, layer.services)

		layerWg := sync.WaitGroup{}
		layerErrorChan := make(chan error, len(layer.services))

		// Use mutex to serialize display updates
		var displayMutex sync.Mutex

		for _, serviceName := range layer.services {
			layerWg.Add(1)
			go func(name string) {
				defer layerWg.Done()

				process := processes[name]
				service := r.cfg.Services[name]

				process.result.UpdateStatus(results.StatusStarting)
				display.UpdateServiceStatus(name, results.StatusStarting)

				// Update display with proper synchronization
				displayMutex.Lock()
				if display.IsBootstrapMode() {
					display.UpdateLayerProgress(layer.level, layer.services)
					time.Sleep(50 * time.Millisecond) // Small delay to prevent rapid updates
				}
				displayMutex.Unlock()

				if err := r.startSingleService(ctx, name, service, process, display, wg); err != nil {
					process.result.UpdateStatus(results.StatusFailed)
					process.result.SetError(err)
					display.UpdateServiceStatus(name, results.StatusFailed)
					display.SetServiceError(name, err)

					displayMutex.Lock()
					if display.IsBootstrapMode() {
						display.UpdateLayerProgress(layer.level, layer.services)
						time.Sleep(50 * time.Millisecond)
					}
					displayMutex.Unlock()

					layerErrorChan <- fmt.Errorf("failed to start service '%s': %w", name, err)
					return
				}

				process.result.UpdateStatus(results.StatusRunning)
				display.UpdateServiceStatus(name, results.StatusRunning)

				displayMutex.Lock()
				if display.IsBootstrapMode() {
					display.UpdateLayerProgress(layer.level, layer.services)
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

func (r *runner) startSingleService(ctx context.Context, name string, service *config.Service, process *serviceProcess, display *ui.Display, wg *sync.WaitGroup) error {
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
		// Completely suppress logging during bootstrap mode
		if !display.IsBootstrapMode() {
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

	// Completely suppress all logging during bootstrap mode
	if !display.IsBootstrapMode() {
		r.log.Info().Msgf("Started service '%s' (PID: %d) in directory: %s", name, cmd.Process.Pid, serviceDir)
	}

	process.cmd = cmd
	process.done = make(chan struct{})

	go r.streamLogs(name, stdout, "STDOUT", display)
	go r.streamLogs(name, stderr, "STDERR", display)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(process.done)
		if err := cmd.Wait(); err != nil {
			process.result.UpdateStatus(results.StatusFailed)
			process.result.SetError(err)
			// Suppress error logging during bootstrap mode
			if !display.IsBootstrapMode() {
				r.log.Error().Err(err).Msgf("Service '%s' exited with error", name)
			}
		}
	}()

	return nil
}

func (r *runner) streamLogs(serviceName string, reader io.Reader, streamType string, display *ui.Display) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, scannerInitialBufferSize), scannerMaxBufferSize)
	for scanner.Scan() {
		line := scanner.Text()
		// Format log line
		var logLine string
		if streamType == "STDERR" {
			logLine = fmt.Sprintf("[%s:ERR] %s", serviceName, line)
		} else {
			logLine = fmt.Sprintf("[%s] %s", serviceName, line)
		}

		// Buffer during bootstrap, print immediately during logs mode
		if display.IsBootstrapMode() {
			display.BufferLog(logLine)
		} else {
			fmt.Println(logLine)
		}
	}
	if err := scanner.Err(); err != nil {
		// Suppress error logging during bootstrap mode
		if !display.IsBootstrapMode() {
			r.log.Error().Err(err).Msgf("Error reading %s stream for service '%s'", streamType, serviceName)
		}
	}
}

func (r *runner) stopAllProcesses(processes map[string]*serviceProcess, display *ui.Display) {
	for _, process := range processes {
		if process.cmd != nil && process.cmd.Process != nil {
			r.log.Info().Msgf("Stopping service '%s' (PID: %d)", process.name, process.cmd.Process.Pid)
			process.result.UpdateStatus(results.StatusStopped)
			display.UpdateServiceStatus(process.name, results.StatusStopped)
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
