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

// workerPool implements the WorkerPool interface
type workerPool struct {
	sem chan struct{}
}

// NewWorkerPool creates a new worker pool with the configured maximum workers
func NewWorkerPool(cfg *config.Config) WorkerPool {
	return &workerPool{
		sem: make(chan struct{}, cfg.Concurrency.Workers),
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
