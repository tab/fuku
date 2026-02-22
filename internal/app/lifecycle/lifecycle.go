package lifecycle

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"fuku/internal/app/errors"
	"fuku/internal/app/process"
	"fuku/internal/config/logger"
)

// Lifecycle handles process group configuration and termination
type Lifecycle interface {
	Configure(cmd *exec.Cmd)
	Terminate(proc process.Process, timeout time.Duration) error
}

// lifecycle implements the Lifecycle interface
type lifecycle struct {
	log logger.Logger
}

// NewLifecycle creates a new Lifecycle instance
func NewLifecycle(log logger.Logger) Lifecycle {
	return &lifecycle{log: log.WithComponent("LIFECYCLE")}
}

// Configure sets up process group for the command
func (l *lifecycle) Configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// Terminate gracefully stops a process and its group
func (l *lifecycle) Terminate(proc process.Process, timeout time.Duration) error {
	cmd := proc.Cmd()
	if cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid
	l.log.Info().Msgf("Stopping service '%s' (PID: %d)", proc.Name(), pid)

	groupErr := l.signalGroup(pid, syscall.SIGTERM)
	if groupErr != nil {
		l.log.Warn().Err(groupErr).Msgf("Failed to send SIGTERM to process group, trying direct signal")
	}

	var directErr error
	if groupErr != nil {
		directErr = cmd.Process.Signal(syscall.SIGTERM)
	}

	if directErr != nil {
		l.log.Error().Err(directErr).Msgf("Failed to send SIGTERM to process '%s'", proc.Name())

		return l.forceKill(proc, pid)
	}

	select {
	case <-proc.Done():
		return nil
	case <-time.After(timeout):
		l.log.Warn().Msgf("Service '%s' did not stop gracefully, forcing kill", proc.Name())
		return l.forceKill(proc, pid)
	}
}

// signalGroup sends a signal to the process group
func (l *lifecycle) signalGroup(pid int, sig syscall.Signal) error {
	return syscall.Kill(-pid, sig)
}

// forceKill sends SIGKILL to the process group
func (l *lifecycle) forceKill(proc process.Process, pid int) error {
	groupErr := syscall.Kill(-pid, syscall.SIGKILL)
	if groupErr != nil {
		l.log.Warn().Err(groupErr).Msgf("Failed to SIGKILL process group, trying direct kill")
	}

	var killErr error
	if groupErr != nil {
		killErr = proc.Cmd().Process.Kill()
	}

	if killErr != nil {
		return fmt.Errorf("%w: %v", errors.ErrFailedToTerminateProcess, killErr)
	}

	<-proc.Done()

	return nil
}
