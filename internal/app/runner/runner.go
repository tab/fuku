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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"fuku/internal/app/readiness"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

const (
	serviceStartupDelay      = 2 * time.Second
	processStopTimeout       = 5 * time.Second
	processKillGracePeriod   = 500 * time.Millisecond
	processStopCheckInterval = 100 * time.Millisecond
	// Exit code offset for signal-based termination (shell convention)
	exitCodeSignalOffset = 128
)

// Runner defines the interface for service orchestration and execution
type Runner interface {
	Run(ctx context.Context, profile string) error
	GetControlChannel() chan ServiceControlRequest
}

// serviceProcess tracks a running service process
type serviceProcess struct {
	name             string
	cmd              *exec.Cmd
	done             chan int
	pid              int
	startTime        time.Time
	readinessChecker readiness.Checker
}

// runner implements the Runner interface
type runner struct {
	cfg              *config.Config
	log              logger.Logger
	readinessFactory readiness.Factory
	callback         EventCallback
	control          chan ServiceControlRequest
}

// NewRunner creates a runner instance with event callback support
func NewRunner(cfg *config.Config, readinessFactory readiness.Factory, log logger.Logger, callback EventCallback) Runner {
	return &runner{
		cfg:              cfg,
		log:              log,
		readinessFactory: readinessFactory,
		callback:         callback,
		control:          make(chan ServiceControlRequest, 10),
	}
}

// GetControlChannel returns the control channel for sending service control requests
func (r *runner) GetControlChannel() chan ServiceControlRequest {
	return r.control
}

// Run executes the specified profile with event callbacks
func (r *runner) Run(ctx context.Context, profile string) error {
	r.emit(EventPhaseStart, PhaseStart{Phase: PhaseDiscovery})

	serviceNames, err := r.cfg.GetServicesForProfile(profile)
	if err != nil {
		r.emitError(fmt.Errorf("failed to resolve profile services: %w", err))
		return err
	}

	services, err := resolveServiceOrder(r.cfg, serviceNames)
	if err != nil {
		r.emitError(fmt.Errorf("failed to resolve service dependencies: %w", err))
		return err
	}

	r.emit(EventPhaseDone, PhaseDone{
		Phase:        PhaseDiscovery,
		ServiceCount: len(services),
		ServiceNames: services,
	})
	r.emit(EventPhaseStart, PhaseStart{Phase: PhaseExecution})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	processes := make([]*serviceProcess, 0, len(services))
	var wg sync.WaitGroup

	batches := groupByDependencyLevel(services, r.cfg)

	for _, batch := range batches {
		for _, serviceName := range batch {
			// Check for shutdown signal during startup
			select {
			case <-ctx.Done():
				r.log.Debug().Msg("Shutdown requested during startup")
				r.emit(EventPhaseStart, PhaseStart{Phase: PhaseShutdown})
				r.stopAllProcesses(processes)
				wg.Wait()
				r.emit(EventPhaseDone, PhaseDone{Phase: PhaseShutdown})
				return nil
			case sig := <-sigChan:
				r.log.Debug().Msgf("Received signal %s during startup", sig)
				r.emit(EventPhaseStart, PhaseStart{Phase: PhaseShutdown})
				r.stopAllProcesses(processes)
				wg.Wait()
				r.emit(EventPhaseDone, PhaseDone{Phase: PhaseShutdown})
				return nil
			default:
			}

			service, exists := r.cfg.Services[serviceName]
			if !exists {
				err := fmt.Errorf("service '%s' not found in configuration", serviceName)
				r.emitError(err)
				r.stopAllProcesses(processes)
				return err
			}

			process, err := r.startService(serviceName, service)
			if err != nil {
				r.emit(EventServiceFail, ServiceFail{
					Name:  serviceName,
					Error: err,
					Time:  time.Now(),
				})
				r.stopAllProcesses(processes)
				return fmt.Errorf("failed to start service '%s': %w", serviceName, err)
			}

			processes = append(processes, process)

			r.emit(EventServiceStart, ServiceStart{
				Name:      process.name,
				PID:       process.pid,
				StartTime: process.startTime,
			})

			if process.readinessChecker != nil {
				wg.Add(1)
				go func(p *serviceProcess) {
					defer wg.Done()
					if err := p.readinessChecker.Check(ctx); err != nil {
						r.log.Warn().Err(err).Msgf("Readiness check failed for service '%s'", p.name)
					} else {
						r.emit(EventServiceReady, ServiceReady{
							Name:      p.name,
							ReadyTime: time.Now(),
						})
					}
				}(process)
			}

			wg.Add(1)
			go func(p *serviceProcess) {
				defer wg.Done()
				exitCode := <-p.done
				r.emit(EventServiceStop, ServiceStop{
					Name:         p.name,
					ExitCode:     exitCode,
					StopTime:     time.Now(),
					GracefulStop: isGracefulStop(exitCode),
				})
			}(process)
		}

		// Wait for services in this batch to initialize
		select {
		case <-ctx.Done():
			r.log.Debug().Msg("Shutdown requested during startup delay")
			r.emit(EventPhaseStart, PhaseStart{Phase: PhaseShutdown})
			r.stopAllProcesses(processes)
			wg.Wait()
			r.emit(EventPhaseDone, PhaseDone{Phase: PhaseShutdown})
			return nil
		case sig := <-sigChan:
			r.log.Debug().Msgf("Received signal %s during startup delay", sig)
			r.emit(EventPhaseStart, PhaseStart{Phase: PhaseShutdown})
			r.stopAllProcesses(processes)
			wg.Wait()
			r.emit(EventPhaseDone, PhaseDone{Phase: PhaseShutdown})
			return nil
		case <-time.After(serviceStartupDelay):
		}
	}

	r.emit(EventPhaseDone, PhaseDone{Phase: PhaseExecution})
	r.emit(EventPhaseStart, PhaseStart{Phase: PhaseRunning})
	r.emit(EventReady, nil)

	processMap := make(map[string]*serviceProcess)
	for _, p := range processes {
		processMap[p.name] = p
	}

	for {
		select {
		case sig := <-sigChan:
			r.log.Debug().Msgf("Received signal %s, shutting down services...", sig)
			r.emit(EventPhaseStart, PhaseStart{Phase: PhaseShutdown})
			cancel()
			time.Sleep(300 * time.Millisecond)
			r.stopAllProcesses(processes)
			wg.Wait()
			r.emit(EventPhaseDone, PhaseDone{Phase: PhaseShutdown})
			return nil

		case <-ctx.Done():
			r.log.Debug().Msg("Context cancelled, shutting down services...")
			r.emit(EventPhaseStart, PhaseStart{Phase: PhaseShutdown})
			time.Sleep(300 * time.Millisecond)
			r.stopAllProcesses(processes)
			wg.Wait()
			r.emit(EventPhaseDone, PhaseDone{Phase: PhaseShutdown})
			return nil

		case req := <-r.control:
			r.handleServiceControl(req, processMap, &processes, &wg)
		}
	}
}

func (r *runner) startService(name string, service *config.Service) (*serviceProcess, error) {
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

	cmd := exec.Command("make", "run")
	cmd.Dir = serviceDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("ENV_FILE=%s", envFile))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

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

	startTime := time.Now()

	checker, err := r.readinessFactory.CreateChecker(service.Readiness)
	if err != nil {
		r.log.Warn().Err(err).Msgf("Failed to create readiness checker for service '%s'", name)
	}

	process := &serviceProcess{
		name:             name,
		cmd:              cmd,
		done:             make(chan int),
		pid:              cmd.Process.Pid,
		startTime:        startTime,
		readinessChecker: checker,
	}

	go r.streamLogs(name, stdout, "STDOUT", process)
	go r.streamLogs(name, stderr, "STDERR", process)

	go func() {
		exitCode := 0
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
				// For Unix systems, we can check if the process was signaled
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					if status.Signaled() {
						// Process was killed by signal: use standard exit code (128 + signal number)
						exitCode = exitCodeSignalOffset + int(status.Signal())
					}
				}
			} else {
				exitCode = 1
			}
		}
		process.done <- exitCode
		close(process.done)
	}()

	return process, nil
}

func (r *runner) streamLogs(serviceName string, reader io.Reader, streamType string, process *serviceProcess) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		r.emit(EventServiceLog, ServiceLog{
			Name:   serviceName,
			Stream: streamType,
			Line:   line,
			Time:   time.Now(),
		})

		if process.readinessChecker != nil {
			if logChecker, ok := process.readinessChecker.(*readiness.LogChecker); ok {
				logChecker.AddLogLine(line)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		r.log.Error().Err(err).Msgf("Error reading %s stream for service '%s'", streamType, serviceName)
	}
}

func (r *runner) stopAllProcesses(processes []*serviceProcess) {
	for i := len(processes) - 1; i >= 0; i-- {
		process := processes[i]
		if process.cmd != nil && process.cmd.Process != nil {
			pid := process.cmd.Process.Pid

			// Check if process is still running using os.Process methods
			if err := process.cmd.Process.Signal(syscall.Signal(0)); err != nil {
				// Process already stopped
				r.killChildProcesses(pid)
				continue
			}

			// Send SIGTERM to process and process group
			if err := process.cmd.Process.Signal(syscall.SIGTERM); err == nil {
				syscall.Kill(-pid, syscall.SIGTERM)
			}

			// Wait for graceful shutdown
			stopped := r.waitForProcessStop(pid, processStopTimeout)
			if !stopped {
				// Force kill if not stopped
				process.cmd.Process.Kill()
				syscall.Kill(-pid, syscall.SIGKILL)
				time.Sleep(processKillGracePeriod)
			}

			r.killChildProcesses(pid)
		}
	}
}

func (r *runner) waitForProcessStop(pid int, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(processStopCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if err := syscall.Kill(pid, 0); err != nil {
				return true
			}
		}
	}
}

func (r *runner) killChildProcesses(parentPID int) {
	// #nosec G204 - ps command with fixed arguments
	cmd := exec.Command("ps", "-A", "-o", "pid=,ppid=")
	output, err := cmd.Output()
	if err != nil {
		r.log.Debug().Err(err).Msg("Failed to list child processes")
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			ppid, _ := strconv.Atoi(fields[1])
			if ppid == parentPID {
				childPID, _ := strconv.Atoi(fields[0])
				r.log.Debug().Msgf("Killing child process %d of parent %d", childPID, parentPID)
				syscall.Kill(childPID, syscall.SIGKILL)
			}
		}
	}
}

func (r *runner) handleServiceControl(req ServiceControlRequest, processMap map[string]*serviceProcess, processes *[]*serviceProcess, wg *sync.WaitGroup) {
	switch req.Action {
	case ControlStop:
		r.stopService(req.ServiceName, processMap)
	case ControlStart:
		r.startServiceControl(req.ServiceName, processMap, processes, wg)
	case ControlRestart:
		r.stopService(req.ServiceName, processMap)
		time.Sleep(1 * time.Second)
		r.startServiceControl(req.ServiceName, processMap, processes, wg)
	}
}

func (r *runner) stopService(serviceName string, processMap map[string]*serviceProcess) {
	process, exists := processMap[serviceName]
	if !exists || process.cmd == nil || process.cmd.Process == nil {
		r.log.Warn().Msgf("Cannot stop service '%s': not running", serviceName)
		return
	}

	// Send SIGTERM to the process using os.Process.Signal for better portability
	if err := process.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		r.log.Warn().Err(err).Msgf("Failed to send SIGTERM to service '%s'", serviceName)
	}

	// Also send SIGTERM to the process group (negative PID) to catch child processes
	pid := process.cmd.Process.Pid
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		r.log.Debug().Err(err).Msgf("Failed to send SIGTERM to process group for service '%s'", serviceName)
	}
}

func (r *runner) startServiceControl(serviceName string, processMap map[string]*serviceProcess, processes *[]*serviceProcess, wg *sync.WaitGroup) {
	service, exists := r.cfg.Services[serviceName]
	if !exists {
		r.log.Error().Msgf("Service '%s' not found in configuration", serviceName)
		return
	}

	process, err := r.startService(serviceName, service)
	if err != nil {
		r.emit(EventServiceFail, ServiceFail{
			Name:  serviceName,
			Error: err,
			Time:  time.Now(),
		})
		return
	}

	processMap[serviceName] = process
	*processes = append(*processes, process)

	r.emit(EventServiceStart, ServiceStart{
		Name:      process.name,
		PID:       process.pid,
		StartTime: process.startTime,
	})

	if process.readinessChecker != nil {
		wg.Add(1)
		go func(p *serviceProcess) {
			defer wg.Done()
			ctx := context.Background()
			if err := p.readinessChecker.Check(ctx); err != nil {
				r.log.Warn().Err(err).Msgf("Readiness check failed for service '%s'", p.name)
			} else {
				r.emit(EventServiceReady, ServiceReady{
					Name:      p.name,
					ReadyTime: time.Now(),
				})
			}
		}(process)
	}

	wg.Add(1)
	go func(p *serviceProcess) {
		defer wg.Done()
		exitCode := <-p.done
		r.emit(EventServiceStop, ServiceStop{
			Name:         p.name,
			ExitCode:     exitCode,
			StopTime:     time.Now(),
			GracefulStop: isGracefulStop(exitCode),
		})
	}(process)
}

func (r *runner) emit(eventType EventType, data interface{}) {
	if r.callback != nil {
		r.callback(Event{
			Type:      eventType,
			Timestamp: time.Now(),
			Data:      data,
		})
	}
}

func (r *runner) emitError(err error) {
	r.emit(EventError, ErrorData{
		Error: err,
		Time:  time.Now(),
	})
}

// isGracefulStop returns true if the exit code indicates the process was terminated by a signal.
// Unix convention: exit codes 128-255 indicate termination by signal (128 + signal number).
// Common signal exit codes:
//   - 128 + syscall.SIGINT (2) = 130 (Ctrl+C)
//   - 128 + syscall.SIGKILL (9) = 137 (force kill)
//   - 128 + syscall.SIGTERM (15) = 143 (graceful termination)
func isGracefulStop(exitCode int) bool {
	// Check specific common signals
	if exitCode == exitCodeSignalOffset+int(syscall.SIGTERM) ||
		exitCode == exitCodeSignalOffset+int(syscall.SIGKILL) ||
		exitCode == exitCodeSignalOffset+int(syscall.SIGINT) {
		return true
	}
	// Any exit code >= 128 indicates termination by signal
	// This covers cases where make or shell wrappers return signal-based exit codes
	return exitCode >= exitCodeSignalOffset
}
