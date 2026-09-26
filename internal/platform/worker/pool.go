package worker

import "context"

// Pool bounds concurrent work to a maximum number of workers
type Pool struct {
	sem chan struct{}
}

// NewPool creates a new worker pool with the configured maximum workers
func NewPool(options Options) *Pool {
	return &Pool{
		sem: make(chan struct{}, options.Workers),
	}
}

// Acquire acquires a worker slot, blocking if all workers are busy or returning error if context is cancelled
func (w *Pool) Acquire(ctx context.Context) error {
	select {
	case w.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases a worker slot
func (w *Pool) Release() {
	<-w.sem
}
