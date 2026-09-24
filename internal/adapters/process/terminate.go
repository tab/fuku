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

	err := h.term()
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
	case <-time.After(h.timeout):
		h.log.Warn(fmt.Sprintf("Service '%s' did not stop gracefully, forcing kill", h.svc.Name))

		return h.forceKill()
	}
}

// term sends SIGTERM to the process group, or to the process itself when the group cannot be signalled
func (h *Handle) term() error {
	// sync: a pid reused between the child's exit and Done() closing would receive the group signal
	groupErr := syscall.Kill(-h.pid, syscall.SIGTERM)
	if groupErr == nil {
		return nil
	}

	h.log.Warn("Failed to send SIGTERM to process group, trying direct signal", "error", groupErr)

	return h.cmd.Process.Signal(syscall.SIGTERM)
}

// forceKill sends SIGKILL to the process group and waits for the exit (a child reaped before the kill needs nothing)
func (h *Handle) forceKill() error {
	// sync: the same pid-reuse window as term
	groupErr := syscall.Kill(-h.pid, syscall.SIGKILL)
	if groupErr != nil {
		h.log.Warn("Failed to SIGKILL process group, trying direct kill", "error", groupErr)
	}

	var killErr error
	if groupErr != nil {
		killErr = h.cmd.Process.Kill()
	}

	if errors.Is(killErr, os.ErrProcessDone) {
		return nil
	}

	if killErr != nil {
		return fmt.Errorf("failed to terminate process: %w", killErr)
	}

	<-h.done

	return nil
}
