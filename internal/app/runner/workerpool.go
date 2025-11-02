package runner

import "fuku/internal/config"

// WorkerPool manages concurrent worker execution with a maximum worker limit
type WorkerPool interface {
	Acquire()
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

// Acquire acquires a worker slot, blocking if all workers are busy
func (w *workerPool) Acquire() {
	w.sem <- struct{}{}
}

// Release releases a worker slot
func (w *workerPool) Release() {
	<-w.sem
}
