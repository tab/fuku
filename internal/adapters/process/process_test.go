package process

import (
	"io"
	"log/slog"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/model"
)

func Test_newHandle(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	svc := model.Service{ID: "test-id-api", Name: "api"}
	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()

	cmd := exec.Command("sleep", "60")
	require.NoError(t, cmd.Start())

	t.Cleanup(func() {
		stdoutWriter.Close()
		stderrWriter.Close()
		cmd.Process.Kill()
		cmd.Wait()
	})

	handle := newHandle(svc, cmd, stdout, stderr, log)

	assert.Equal(t, svc, handle.Service())
	assert.Equal(t, cmd.Process.Pid, handle.PID())
	assert.Equal(t, stdout, handle.Stdout())
	assert.Equal(t, stderr, handle.Stderr())
	assert.Equal(t, ShutdownTimeout, handle.timeout)
	assert.Equal(t, log, handle.log)
	assert.False(t, handle.exited())

	close(handle.done)

	assert.True(t, handle.exited())

	_, open := <-handle.Done()
	assert.False(t, open)
}
