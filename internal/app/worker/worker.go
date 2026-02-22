package worker

import (
	"context"

	"fuku/internal/config"
)

// Pool manages concurrent worker execution with a maximum worker limit
type Pool interface {
	Acquire(ctx context.Context) error
	Release()
}

// pool implements the Pool interface
type pool struct {
	sem chan struct{}
}

// NewWorkerPool creates a new worker pool with the configured maximum workers
func NewWorkerPool(cfg *config.Config) Pool {
	return &pool{
		sem: make(chan struct{}, cfg.Concurrency.Workers),
	}
}

// Acquire acquires a worker slot, blocking if all workers are busy or returning error if context is cancelled
func (w *pool) Acquire(ctx context.Context) error {
	select {
	case w.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases a worker slot
func (w *pool) Release() {
	<-w.sem
}
