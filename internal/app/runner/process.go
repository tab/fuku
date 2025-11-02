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

type process struct {
	name         string
	cmd          *exec.Cmd
	done         chan struct{}
	ready        chan error
	stdoutReader *io.PipeReader
	stderrReader *io.PipeReader
}

func (p *process) Name() string {
	return p.name
}

func (p *process) Cmd() *exec.Cmd {
	return p.cmd
}

func (p *process) Done() <-chan struct{} {
	return p.done
}

func (p *process) Ready() <-chan error {
	return p.ready
}

func (p *process) SignalReady(err error) {
	if err != nil {
		select {
		case p.ready <- err:
		default:
		}
	}

	close(p.ready)
}

func (p *process) StdoutReader() *io.PipeReader {
	return p.stdoutReader
}

func (p *process) StderrReader() *io.PipeReader {
	return p.stderrReader
}
