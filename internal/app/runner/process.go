package runner

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

// process implements the Process interface
type process struct {
	name         string
	cmd          *exec.Cmd
	done         chan struct{}
	ready        chan error
	stdoutReader *io.PipeReader
	stderrReader *io.PipeReader
}

// Name returns the service name
func (p *process) Name() string {
	return p.name
}

// Cmd returns the underlying exec command
func (p *process) Cmd() *exec.Cmd {
	return p.cmd
}

// Done returns a channel that closes when the process exits
func (p *process) Done() <-chan struct{} {
	return p.done
}

// Ready returns a channel that receives when the process is ready
func (p *process) Ready() <-chan error {
	return p.ready
}

// SignalReady signals the ready channel with optional error
func (p *process) SignalReady(err error) {
	if err != nil {
		select {
		case p.ready <- err:
		default:
		}
	}

	close(p.ready)
}

// StdoutReader returns the stdout pipe reader
func (p *process) StdoutReader() *io.PipeReader {
	return p.stdoutReader
}

// StderrReader returns the stderr pipe reader
func (p *process) StderrReader() *io.PipeReader {
	return p.stderrReader
}
