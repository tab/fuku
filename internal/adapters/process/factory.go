package process

import (
	"errors"
	"fmt"
	"io"
	"os/exec"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Logger is the logging surface the process adapter writes through
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Factory creates the child processes services run (Fx builds it once and it creates a handle per launch)
type Factory struct {
	tracker *Tracker
	sink    LogSink
	log     Logger
}

// NewFactory creates the process factory
func NewFactory(tracker *Tracker, sink LogSink, log Logger) *Factory {
	return &Factory{
		tracker: tracker,
		sink:    sink,
		log:     log,
	}
}

// Start launches the service command and tracks its handle as one operation, so no live child is ever untracked
func (f *Factory) Start(svc model.Service) (contracts.Process, error) {
	stdoutReader, stdoutPipe := io.Pipe()
	stderrReader, stderrPipe := io.Pipe()

	stdout := f.newStreamWriter(stdoutPipe, svc, streamStdout)
	stderr := f.newStreamWriter(stderrPipe, svc, streamStderr)

	prepared, err := prepare(svc.Command, svc.Directory, stdout, stderr)
	if err != nil {
		return nil, err
	}

	handle, err := f.tracker.track(func() (*Handle, error) {
		if err := prepared.cmd.Start(); err != nil {
			return nil, fmt.Errorf("%w: %w", contracts.ErrFailedToStartCommand, err)
		}

		return newHandle(svc, prepared.cmd, stdoutReader, stderrReader, f.log), nil
	})
	if err != nil {
		return nil, err
	}

	f.log.Info(fmt.Sprintf("Started service '%s' (PID: %d) in directory: %s", svc.Name, handle.pid, prepared.dir))

	go f.wait(handle, stdout, stderr)

	return handle, nil
}

// wait reaps the child, closes its streams and then marks the handle done
func (f *Factory) wait(handle *Handle, stdout, stderr *streamWriter) {
	defer close(handle.done)

	err := handle.cmd.Wait()

	// sync: Wait returns once exec stopped writing the streams, so closing them after it keeps the last line
	stdout.Close()
	stderr.Close()

	if err != nil && !errors.Is(err, exec.ErrWaitDelay) {
		f.log.Error(fmt.Sprintf("Service '%s' exited with error", handle.svc.Name), "error", err)
	}
}
