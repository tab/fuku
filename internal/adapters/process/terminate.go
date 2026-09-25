package process

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// Terminate stops the child once: the first call signals it and every other call returns that call's result
func (h *Handle) Terminate() error {
	h.terminate.Do(func() {
		h.terminated = h.stop()
	})

	return h.terminated
}

// stop asks the process group to exit and kills it when it has not within the timeout (an exited child needs nothing)
func (h *Handle) stop() error {
	if h.exited() {
		return nil
	}

	h.log.Info(fmt.Sprintf("Stopping service '%s' (PID: %d)", h.svc.Name, h.pid))

	// sync: a pid reused between the child's exit and Done() closing would receive the group signal
	err := signalGroup(h.pid, h.cmd.Process, syscall.SIGTERM)
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}

	if err != nil {
		h.log.Error(fmt.Sprintf("Failed to send SIGTERM to process '%s'", h.svc.Name), "error", err)

		return h.forceKill()
	}

	select {
	case <-h.done:
		return nil
	case <-time.After(ShutdownTimeout):
		h.log.Warn(fmt.Sprintf("Service '%s' did not stop gracefully, forcing kill", h.svc.Name))

		return h.forceKill()
	}
}

// forceKill sends SIGKILL to the process group and waits for the exit (a child reaped before the kill needs nothing)
func (h *Handle) forceKill() error {
	// sync: the same pid-reuse window as the SIGTERM in stop
	err := signalGroup(h.pid, h.cmd.Process, syscall.SIGKILL)
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to terminate process: %w", err)
	}

	<-h.done

	return nil
}

// signalGroup sends sig to the process group of pid, or to proc itself when the group cannot be signalled
func signalGroup(pid int, proc *os.Process, sig syscall.Signal) error {
	if err := syscall.Kill(-pid, sig); err == nil {
		return nil
	}

	return proc.Signal(sig)
}
