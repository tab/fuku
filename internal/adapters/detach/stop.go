package detach

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	"fuku/internal/contracts"
)

// stopPollInterval is how often the stop checks whether the instance has exited
const stopPollInterval = 100 * time.Millisecond

// StatusSource reads the status of the running instance
type StatusSource interface {
	Status() (contracts.LogStatus, error)
}

// Stopper stops the running instance of the project: SIGTERM, a bounded wait, then SIGKILL
type Stopper struct {
	source  StatusSource
	options StopOptions
	stdout  io.Writer
}

// NewStopper creates the stopper of the running instance
func NewStopper(source StatusSource, options StopOptions, stdout io.Writer) *Stopper {
	return &Stopper{source: source, options: options, stdout: stdout}
}

// Stop signals the running instance and waits for it to exit; it returns nil when no instance answers
func (s *Stopper) Stop(ctx context.Context) error {
	status, err := s.source.Status()
	if errors.Is(err, contracts.ErrNoInstanceRunning) {
		return nil
	}

	if err != nil {
		fmt.Fprintf(s.stdout, "Cannot reach the running fuku: %v\n", err)

		return nil
	}

	if status.PID == 0 {
		fmt.Fprintln(s.stdout, "The running fuku reports no process ID; restart it with this version to stop it")

		return nil
	}

	fmt.Fprintf(s.stdout, "Stopping fuku · pid %d · profile %s ... ", status.PID, status.Profile)

	started := time.Now()

	if err := syscall.Kill(status.PID, syscall.SIGTERM); err != nil {
		fmt.Fprintln(s.stdout, "failed")

		return fmt.Errorf("failed to stop fuku (pid %d): %w", status.PID, err)
	}

	exited, err := s.wait(ctx, status.PID)
	if err != nil {
		fmt.Fprintln(s.stdout, "interrupted")

		return err
	}

	if exited {
		fmt.Fprintf(s.stdout, "stopped in %s\n", seconds(time.Since(started)))

		return nil
	}

	if err := syscall.Kill(status.PID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		fmt.Fprintln(s.stdout, "failed")

		return fmt.Errorf("failed to kill fuku (pid %d): %w", status.PID, err)
	}

	fmt.Fprintf(s.stdout, "killed after %s\n", seconds(s.options.Timeout))

	return nil
}

// wait polls until the process is gone, returning false when the timeout passes first and an error when ctx ends
func (s *Stopper) wait(ctx context.Context, pid int) (bool, error) {
	deadline := time.NewTimer(s.options.Timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(stopPollInterval)
	defer ticker.Stop()

	for {
		if exited(pid) {
			return true, nil
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline.C:
			return false, nil
		case <-ticker.C:
		}
	}
}

// exited reports a process that is gone, or a zombie its parent has not reaped yet
func exited(pid int) bool {
	if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
		return true
	}

	proc, err := process.NewProcess(int32(pid)) // #nosec G115 -- PID fits in int32
	if err != nil {
		return true
	}

	status, err := proc.Status()

	return err == nil && slices.Contains(status, process.Zombie)
}
