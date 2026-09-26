package e2e

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Signal_InterruptDuringStartup(t *testing.T) {
	runner := NewRunner(t, "testdata/readiness")
	defer runner.Stop()

	require.NoError(t, runner.Start("slow"))
	require.NoError(t, runner.WaitForLog("service_starting", 15*time.Second))

	pid, err := runner.ServicePID("slow", 10*time.Second)
	require.NoError(t, err)

	defer syscall.Kill(-pid, syscall.SIGKILL)

	require.NoError(t, runner.Signal(syscall.SIGINT))

	assert.Equal(t, 0, runner.ExitCode())
	assert.Empty(t, runner.Stderr())

	output := runner.Output()

	assert.Contains(t, output, "signal signal=interrupt")
	assert.Regexp(t, `service_stopped .*service=slow`, output)
	assert.Contains(t, output, "phase=stopped")
	assert.NotContains(t, output, "phase=running")

	require.NoError(t, WaitForGroupExit(pid, 10*time.Second))
}

func Test_Signal_ShutdownAfterReady(t *testing.T) {
	tests := []struct {
		name     string
		signal   os.Signal
		expected string
	}{
		{name: "SIGINT", signal: syscall.SIGINT, expected: "signal signal=interrupt"},
		{name: "SIGTERM", signal: syscall.SIGTERM, expected: "signal signal=terminated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(t, "testdata/default-tier")
			defer runner.Stop()

			require.NoError(t, runner.Start("default"))
			require.NoError(t, runner.WaitForRunning(15*time.Second))
			require.NoError(t, runner.WaitForLogCount("Service ready", 2, 15*time.Second))

			authPID, err := runner.ServicePID("auth-api", 10*time.Second)
			require.NoError(t, err)

			userPID, err := runner.ServicePID("user-api", 10*time.Second)
			require.NoError(t, err)

			defer syscall.Kill(-authPID, syscall.SIGKILL)
			defer syscall.Kill(-userPID, syscall.SIGKILL)

			require.NoError(t, runner.Signal(tt.signal))

			assert.Equal(t, 0, runner.ExitCode())
			assert.Empty(t, runner.Stderr())

			output := runner.Output()

			assert.Contains(t, output, tt.expected)
			assert.Regexp(t, `service_stopped .*service=auth-api`, output)
			assert.Regexp(t, `service_stopped .*service=user-api`, output)
			assert.Contains(t, output, "phase=stopped")

			require.NoError(t, WaitForGroupExit(authPID, 10*time.Second))
			require.NoError(t, WaitForGroupExit(userPID, 10*time.Second))
		})
	}
}
