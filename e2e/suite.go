package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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

// StartWithConfig launches fuku with --config flag and given profile
func (r *Runner) StartWithConfig(configFile, profile string) error {
	bin := os.Getenv("FUKU_BIN")
	if bin == "" {
		bin = "fuku"
	}

	args := []string{"--config", configFile, "run", profile, "--no-ui"}
	r.cmd = exec.Command(bin, args...)
	r.cmd.Dir = r.workDir
	r.cmd.Stdout = r.stdout
	r.cmd.Stderr = r.stderr

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start fuku: %w", err)
	}

	return nil
}

// StartWith launches fuku with the given arguments
func (r *Runner) StartWith(args ...string) error {
	bin := os.Getenv("FUKU_BIN")
	if bin == "" {
		bin = "fuku"
	}

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
	return r.Signal(syscall.SIGTERM)
}

// Signal sends sig and waits for fuku to exit, killing it after 10s
func (r *Runner) Signal(sig os.Signal) error {
	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	if err := r.cmd.Process.Signal(sig); err != nil {
		return fmt.Errorf("failed to send %s: %w", sig, err)
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

		return errors.New("process did not exit gracefully, killed")
	}
}

// WaitForLog blocks until pattern appears in stdout or timeout
func (r *Runner) WaitForLog(pattern string, timeout time.Duration) error {
	return r.WaitForLogCount(pattern, 1, timeout)
}

// WaitForLogCount blocks until pattern appears at least count times in stdout or timeout
func (r *Runner) WaitForLogCount(pattern string, count int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %d occurrences of log pattern %q\nOutput:\n%s", count, pattern, r.Output())
		case <-ticker.C:
			if strings.Count(r.Output(), pattern) >= count {
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

// ServicePID waits for a service_starting event of the service and returns its latest PID, also its process group
func (r *Runner) ServicePID(service string, timeout time.Duration) (int, error) {
	pattern := regexp.MustCompile(fmt.Sprintf(`service_starting .*pid=(\d+) service=%s\s`, regexp.QuoteMeta(service)))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		matches := pattern.FindAllStringSubmatch(r.Output(), -1)
		if len(matches) > 0 {
			return strconv.Atoi(matches[len(matches)-1][1])
		}

		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("timeout waiting for service_starting of %q\nOutput:\n%s", service, r.Output())
		case <-ticker.C:
		}
	}
}

// WaitForGroupExit blocks until no process of the group is left or timeout
func WaitForGroupExit(pgid int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if errors.Is(syscall.Kill(-pgid, 0), syscall.ESRCH) {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for process group %d to exit", pgid)
		case <-ticker.C:
		}
	}
}

// WaitForNoProcess blocks until no process command line matches pattern or timeout
func WaitForNoProcess(pattern string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		var exitErr *exec.ExitError
		if err := exec.Command("pgrep", "-f", pattern).Run(); errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for no process to match %q", pattern)
		case <-ticker.C:
		}
	}
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

// RunResult holds output from a completed fuku command
type RunResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// RunOnce executes fuku with the given args and waits for completion
func RunOnce(t *testing.T, dir string, args ...string) RunResult {
	t.Helper()

	bin := os.Getenv("FUKU_BIN")
	if bin == "" {
		bin = "fuku"
	}

	workDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	cmd := exec.Command(bin, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	exitCode := 0

	var exitErr *exec.ExitError

	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		exitCode = exitErr.ExitCode()
	default:
		t.Fatalf("failed to run command: %v", err)
	}

	return RunResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	return strings.Index(s, substr)
}

// SocketPath returns the socket of the fuku instance serving dir, named by the fingerprint the binary computes
func SocketPath(t *testing.T, dir string) string {
	t.Helper()

	workDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	project, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	sum := sha256.Sum256([]byte(project))

	return filepath.Join("/tmp", "fuku-"+hex.EncodeToString(sum[:])[:16]+".sock")
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

	args := make([]string, 0, 4+len(services))
	args = append(args, "logs")

	if profile != "" {
		args = append(args, "--profile", profile)
	}

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
