package e2e

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

const detachedDir = "testdata/detached"

// detachedPID reads the instance PID from the summary of a detached start
func detachedPID(t *testing.T, stdout string) int {
	t.Helper()

	match := regexp.MustCompile(`Running detached · pid (\d+) ·`).FindStringSubmatch(stdout)
	require.Len(t, match, 2, "no summary in:\n%s", stdout)

	pid, err := strconv.Atoi(match[1])
	require.NoError(t, err)

	return pid
}

// endDetached stops an instance a failed test left behind; the instance leads its own session, so its PID is its group
func endDetached(pid int) {
	syscall.Kill(pid, syscall.SIGTERM)

	if WaitForGroupExit(pid, 10*time.Second) != nil {
		syscall.Kill(pid, syscall.SIGKILL)
	}
}

func Test_Detached_RunsUntilStopped(t *testing.T) {
	result := RunOnce(t, detachedDir, "run", "-d")

	pid := detachedPID(t, result.Stdout)
	defer endDetached(pid)

	assert.Equal(t, 0, result.ExitCode)
	assert.Regexp(t, `✔ api Ready \d+\.\ds`, result.Stdout)
	assert.Contains(t, result.Stdout, "✔ worker Ready")
	assert.Regexp(t, `Running detached · pid \d+ · 2 services · \d+\.\ds`, result.Stdout)
	assert.Contains(t, result.Stdout, "API 127.0.0.1:19890")
	assert.NotContains(t, result.Stdout, "\x1b", "a pipe must never receive escape sequences")
	assert.Empty(t, result.Stderr)

	require.NoError(t, exec.Command("pgrep", "-f", "sleep 601").Run(), "the services must keep running after the command returns")

	sid, err := unix.Getsid(pid)
	require.NoError(t, err)
	assert.Equal(t, pid, sid, "the detached instance must lead its own session")

	logs := RunOnce(t, detachedDir, "logs", "--no-ui", "--no-follow")

	assert.Equal(t, 0, logs.ExitCode, "the detached instance must serve its socket")
	assert.Contains(t, logs.Stdout, "phase=running")

	stop := RunOnce(t, detachedDir, "stop")

	assert.Equal(t, 0, stop.ExitCode)
	assert.Regexp(t, `Stopping fuku · pid `+strconv.Itoa(pid)+` · profile default \.\.\. stopped in \d+\.\ds`, stop.Stdout)
	require.NoError(t, WaitForGroupExit(pid, 10*time.Second))
	assert.NoError(t, WaitForNoProcess("sleep 601", 10*time.Second))
}

func Test_Detached_FailedServiceStopsEverything(t *testing.T) {
	result := RunOnce(t, detachedDir, "run", "broken", "-d")

	assert.Equal(t, 1, result.ExitCode)
	assert.Contains(t, result.Stdout, "✔ cache Ready")
	assert.Contains(t, result.Stdout, "✗ broken Failed")
	assert.Contains(t, result.Stderr, "Error: service 'broken' failed to start")
	assert.NotContains(t, result.Stdout, "Running detached")
	assert.NoError(t, WaitForNoProcess("sleep 602", 10*time.Second))
	assert.NoError(t, WaitForNoProcess("sleep 603", 10*time.Second))

	logs := RunOnce(t, detachedDir, "logs", "--no-ui", "--no-follow")

	assert.NotEqual(t, 0, logs.ExitCode, "a failed detached start must leave no instance running")
}

func Test_Detached_RefusesSecondRun(t *testing.T) {
	first := RunOnce(t, detachedDir, "run", "-d")

	pid := detachedPID(t, first.Stdout)
	defer endDetached(pid)

	second := RunOnce(t, detachedDir, "run", "-d")

	assert.Equal(t, 1, second.ExitCode)
	assert.Contains(t, second.Stderr, "fuku is already running for this project")

	logs := RunOnce(t, detachedDir, "logs", "--no-ui", "--no-follow")

	assert.Equal(t, 0, logs.ExitCode, "the refused start must leave the first instance running")
}

func Test_Detached_RelativeConfig(t *testing.T) {
	result := RunOnce(t, "testdata", "--config", "detached/fuku.yaml", "run", "-d")

	pid := detachedPID(t, result.Stdout)
	defer endDetached(pid)

	assert.Equal(t, 0, result.ExitCode)

	stop := RunOnce(t, "testdata", "--config", "detached/fuku.yaml", "stop")

	assert.Equal(t, 0, stop.ExitCode)
	assert.Contains(t, stop.Stdout, "stopped in")
	require.NoError(t, WaitForGroupExit(pid, 10*time.Second))
}

func Test_Detached_InterruptStopsEverything(t *testing.T) {
	runner := NewRunner(t, detachedDir)

	require.NoError(t, runner.StartWith("run", "hang", "-d"))
	require.NoError(t, runner.WaitForLog("✔ cache Ready", 30*time.Second))
	require.NoError(t, runner.WaitForLog("• hang Starting", 30*time.Second))
	require.NoError(t, runner.Signal(os.Interrupt))

	assert.Equal(t, 130, runner.ExitCode())
	assert.NoError(t, WaitForNoProcess("sleep 602", 10*time.Second))
	assert.NoError(t, WaitForNoProcess("sleep 604", 10*time.Second))

	logs := RunOnce(t, detachedDir, "logs", "--no-ui", "--no-follow")

	assert.NotEqual(t, 0, logs.ExitCode, "an aborted detached start must leave no instance running")
}
