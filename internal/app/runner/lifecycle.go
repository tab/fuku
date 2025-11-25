package runner

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"fuku/internal/app/errors"
	"fuku/internal/config/logger"
)

// Lifecycle handles process group configuration and termination
type Lifecycle interface {
	Configure(cmd *exec.Cmd)
	Terminate(proc Process, timeout time.Duration) error
}

type lifecycle struct {
	log logger.Logger
}

// NewLifecycle creates a new Lifecycle instance
func NewLifecycle(log logger.Logger) Lifecycle {
	return &lifecycle{log: log}
}

// Configure sets up process group for the command
func (l *lifecycle) Configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// Terminate gracefully stops a process and its group
func (l *lifecycle) Terminate(proc Process, timeout time.Duration) error {
	cmd := proc.Cmd()
	if cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid
	l.log.Info().Msgf("Stopping service '%s' (PID: %d)", proc.Name(), pid)

	if err := l.signalGroup(pid, syscall.SIGTERM); err != nil {
		l.log.Warn().Err(err).Msgf("Failed to send SIGTERM to process group, trying direct signal")

		if directErr := cmd.Process.Signal(syscall.SIGTERM); directErr != nil {
			l.log.Error().Err(directErr).Msgf("Failed to send SIGTERM to process '%s'", proc.Name())

			return l.forceKill(proc, pid)
		}
	}

	select {
	case <-proc.Done():
		return nil
	case <-time.After(timeout):
		l.log.Warn().Msgf("Service '%s' did not stop gracefully, forcing kill", proc.Name())
		return l.forceKill(proc, pid)
	}
}

func (l *lifecycle) signalGroup(pid int, sig syscall.Signal) error {
	return syscall.Kill(-pid, sig)
}

func (l *lifecycle) forceKill(proc Process, pid int) error {
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		l.log.Warn().Err(err).Msgf("Failed to SIGKILL process group, trying direct kill")

		if killErr := proc.Cmd().Process.Kill(); killErr != nil {
			return fmt.Errorf("%w: %v", errors.ErrFailedToTerminateProcess, killErr)
		}
	}

	<-proc.Done()

	return nil
}
