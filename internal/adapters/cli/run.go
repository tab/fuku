package cli

import "context"

// Runtime is the running profile the headless command waits on
type Runtime interface {
	Done() <-chan struct{}
}

// Run is the headless run command: it waits until the services runtime has finished the profile
type Run struct {
	runtime Runtime
}

// NewRun creates the headless run command
func NewRun(runtime Runtime) *Run {
	return &Run{runtime: runtime}
}

// Run waits for the runtime or the context to end and returns 0 (a failure reaches the arbiter via Reporter.Fail)
func (r *Run) Run(ctx context.Context) (int, error) {
	select {
	case <-r.runtime.Done():
	case <-ctx.Done():
	}

	return 0, nil
}
