package process

import (
	"io"
	"os/exec"
)

// Process represents a running service process
type Process interface {
	Name() string
	Cmd() *exec.Cmd
	Done() <-chan struct{}
	Ready() <-chan error
	SignalReady(err error)
	StdoutReader() *io.PipeReader
	StderrReader() *io.PipeReader
}

// Handle wraps a Process with lifecycle control methods
type Handle struct {
	Process
	done chan struct{}
}

// Close signals that the process has exited
func (h *Handle) Close() {
	close(h.done)
}

// Params contains parameters for creating a new process
type Params struct {
	Name         string
	Cmd          *exec.Cmd
	StdoutReader *io.PipeReader
	StderrReader *io.PipeReader
}

// proc implements the Process interface
type proc struct {
	name         string
	cmd          *exec.Cmd
	done         chan struct{}
	ready        chan error
	stdoutReader *io.PipeReader
	stderrReader *io.PipeReader
}

// New creates a new Process instance and returns a Handle for lifecycle control
func New(p Params) *Handle {
	done := make(chan struct{})
	process := &proc{
		name:         p.Name,
		cmd:          p.Cmd,
		done:         done,
		ready:        make(chan error, 1),
		stdoutReader: p.StdoutReader,
		stderrReader: p.StderrReader,
	}

	return &Handle{
		Process: process,
		done:    done,
	}
}

// Name returns the service name
func (p *proc) Name() string {
	return p.name
}

// Cmd returns the underlying exec command
func (p *proc) Cmd() *exec.Cmd {
	return p.cmd
}

// Done returns a channel that closes when the process exits
func (p *proc) Done() <-chan struct{} {
	return p.done
}

// Ready returns a channel that receives when the process is ready
func (p *proc) Ready() <-chan error {
	return p.ready
}

// SignalReady signals the ready channel with optional error
func (p *proc) SignalReady(err error) {
	if err != nil {
		select {
		case p.ready <- err:
		default:
		}
	}

	close(p.ready)
}

// StdoutReader returns the stdout pipe reader
func (p *proc) StdoutReader() *io.PipeReader {
	return p.stdoutReader
}

// StderrReader returns the stderr pipe reader
func (p *proc) StderrReader() *io.PipeReader {
	return p.stderrReader
}
