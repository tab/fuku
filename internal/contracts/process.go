package contracts

import (
	"io"

	"fuku/internal/model"
)

// Process is the live child of one service (the process adapter tracks it, readiness observes it, services drives it)
type Process interface {
	Service() model.Service
	PID() int
	Done() <-chan struct{}
	Stdout() io.Reader
	Stderr() io.Reader
	Terminate() error
}
