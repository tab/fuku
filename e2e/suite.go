package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// lockedBuffer is a thread-safe bytes.Buffer for capturing process output
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write implements io.Writer with mutex protection
func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

// String returns the buffer contents with mutex protection
func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// Runner manages fuku process for e2e tests
type Runner struct {
	t       *testing.T
	cmd     *exec.Cmd
	stdout  *lockedBuffer
	stderr  *lockedBuffer
	workDir string
}

// NewRunner creates runner for a test case directory
func NewRunner(t *testing.T, dir string) *Runner {
	t.Helper()

	workDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	return &Runner{
		t:       t,
		workDir: workDir,
		stdout:  &lockedBuffer{},
		stderr:  &lockedBuffer{},
	}
}

// Start launches fuku with given profile and --no-ui flag
func (r *Runner) Start(profile string) error {
	bin := os.Getenv("FUKU_BIN")
	if bin == "" {
		bin = "fuku"
	}

	args := []string{"run", profile, "--no-ui"}
	r.cmd = exec.Command(bin, args...)
	r.cmd.Dir = r.workDir
	r.cmd.Stdout = r.stdout
	r.cmd.Stderr = r.stderr

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start fuku: %w", err)
	}

	return nil
}

// Stop sends SIGTERM and waits for graceful shutdown
func (r *Runner) Stop() error {
	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	if err := r.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	done := make(chan error, 1)

	go func() {
		done <- r.cmd.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-time.After(10 * time.Second):
		r.cmd.Process.Kill()
		<-done

		return fmt.Errorf("process did not exit gracefully, killed")
	}
}

// WaitForLog blocks until pattern appears in stdout or timeout
func (r *Runner) WaitForLog(pattern string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for log pattern %q\nOutput:\n%s", pattern, r.Output())
		case <-ticker.C:
			if strings.Contains(r.Output(), pattern) {
				return nil
			}
		}
	}
}

// WaitForServiceStarted waits for a service to be started
func (r *Runner) WaitForServiceStarted(service string, timeout time.Duration) error {
	pattern := fmt.Sprintf(`Started service '%s'`, service)
	return r.WaitForLog(pattern, timeout)
}

// WaitForTierReady waits for a tier to be fully started
func (r *Runner) WaitForTierReady(tier string, timeout time.Duration) error {
	pattern := fmt.Sprintf(`Tier '%s' started successfully`, tier)
	return r.WaitForLog(pattern, timeout)
}

// WaitForRunning waits for the startup phase to complete
func (r *Runner) WaitForRunning(timeout time.Duration) error {
	return r.WaitForLog("Startup phase complete", timeout)
}

// TouchFile modifies a file to trigger watcher
func (r *Runner) TouchFile(path string) error {
	fullPath := filepath.Join(r.workDir, path)

	file, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	return os.WriteFile(fullPath, content, file.Mode())
}

// Output returns current stdout content
func (r *Runner) Output() string {
	return r.stdout.String()
}

// Stderr returns current stderr content
func (r *Runner) Stderr() string {
	return r.stderr.String()
}

// ExitCode returns process exit code (after Stop)
func (r *Runner) ExitCode() int {
	if r.cmd == nil || r.cmd.ProcessState == nil {
		return -1
	}

	return r.cmd.ProcessState.ExitCode()
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	return strings.Index(s, substr)
}

// LogsRunner manages fuku logs command for e2e tests
type LogsRunner struct {
	t       *testing.T
	cmd     *exec.Cmd
	stdout  *lockedBuffer
	stderr  *lockedBuffer
	workDir string
}

// NewLogsRunner creates a runner for fuku logs command
func NewLogsRunner(t *testing.T, dir string) *LogsRunner {
	t.Helper()

	workDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	return &LogsRunner{
		t:       t,
		workDir: workDir,
		stdout:  &lockedBuffer{},
		stderr:  &lockedBuffer{},
	}
}

// Start launches fuku logs command
func (r *LogsRunner) Start(profile string, services ...string) error {
	bin := os.Getenv("FUKU_BIN")
	if bin == "" {
		bin = "fuku"
	}

	args := []string{"logs", profile}
	args = append(args, services...)

	r.cmd = exec.Command(bin, args...)
	r.cmd.Dir = r.workDir
	r.cmd.Stdout = r.stdout
	r.cmd.Stderr = r.stderr

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start fuku logs: %w", err)
	}

	return nil
}

// Stop terminates the logs command
func (r *LogsRunner) Stop() error {
	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	r.cmd.Process.Kill()
	r.cmd.Wait()

	return nil
}

// Output returns current stdout content
func (r *LogsRunner) Output() string {
	return r.stdout.String()
}

// WaitForLog blocks until pattern appears in stdout or timeout
func (r *LogsRunner) WaitForLog(pattern string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for log pattern %q\nOutput:\n%s", pattern, r.Output())
		case <-ticker.C:
			if strings.Contains(r.Output(), pattern) {
				return nil
			}
		}
	}
}
