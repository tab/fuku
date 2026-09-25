package process

import (
	"io"
	"os/exec"
	"sync"
	"time"

	"fuku/internal/model"
)

// ShutdownTimeout bounds the wait for a child to exit after it is asked to stop
const ShutdownTimeout = 5 * time.Second

// Handle is the tracked child of one service
type Handle struct {
	svc        model.Service
	cmd        *exec.Cmd
	pid        int
	stdout     *io.PipeReader
	stderr     *io.PipeReader
	done       chan struct{}
	terminate  sync.Once
	terminated error
	log        Logger
}

// newHandle wraps a started command (its readers carry the streams the stream writers feed)
func newHandle(svc model.Service, cmd *exec.Cmd, stdout, stderr *io.PipeReader, log Logger) *Handle {
	return &Handle{
		svc:    svc,
		cmd:    cmd,
		pid:    cmd.Process.Pid,
		stdout: stdout,
		stderr: stderr,
		done:   make(chan struct{}),
		log:    log,
	}
}

// Service returns the service the child runs
func (h *Handle) Service() model.Service {
	return h.svc
}

// PID returns the child's process ID
func (h *Handle) PID() int {
	return h.pid
}

// Done returns a channel that closes once the child has exited and its streams are closed
func (h *Handle) Done() <-chan struct{} {
	return h.done
}

// Stdout returns the child's standard output
func (h *Handle) Stdout() io.Reader {
	return h.stdout
}

// Stderr returns the child's standard error
func (h *Handle) Stderr() io.Reader {
	return h.stderr
}

// exited reports whether the child has already exited
func (h *Handle) exited() bool {
	select {
	case <-h.done:
		return true
	default:
		return false
	}
}
