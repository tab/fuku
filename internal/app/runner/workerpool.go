package runner

import (
	"context"

	"fuku/internal/config"
)

// WorkerPool manages concurrent worker execution with a maximum worker limit
type WorkerPool interface {
	Acquire(ctx context.Context) error
	Release()
}

type workerPool struct {
	sem chan struct{}
}

// NewWorkerPool creates a new worker pool with the specified maximum workers
func NewWorkerPool() WorkerPool {
	return &workerPool{
		sem: make(chan struct{}, config.MaxWorkers),
	}
}

// Acquire acquires a worker slot, blocking if all workers are busy or returning error if context is cancelled
func (w *workerPool) Acquire(ctx context.Context) error {
	select {
	case w.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases a worker slot
func (w *workerPool) Release() {
	<-w.sem
}
