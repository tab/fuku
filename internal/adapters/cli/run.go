package cli

import (
	"context"
	"errors"
)

// Runtime is the running profile the headless command waits on
type Runtime interface {
	Done() <-chan struct{}
	Err() error
}

// Run is the headless run command: it waits until the services runtime has finished the profile
type Run struct {
	runtime Runtime
}

// NewRun creates the headless run command
func NewRun(runtime Runtime) *Run {
	return &Run{runtime: runtime}
}

// Run waits for the runtime and returns 1 when the run failed (a cancelled run is a clean exit)
func (r *Run) Run(ctx context.Context) (int, error) {
	select {
	case <-r.runtime.Done():
	case <-ctx.Done():
		return 0, nil
	}

	if err := r.runtime.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return 1, err
	}

	return 0, nil
}
