package api

import "sync/atomic"

// ConnectionTracker tracks concurrent streaming connections against a limit
type ConnectionTracker struct {
	count atomic.Int32
	max   int32
}

// NewConnectionTracker creates a new connection tracker with the given limit
func NewConnectionTracker(limit int) *ConnectionTracker {
	return &ConnectionTracker{max: int32(limit)} //nolint:gosec // config-validated positive int, no overflow risk
}

// Acquire attempts to claim a connection slot, returns false if at limit
func (t *ConnectionTracker) Acquire() bool {
	for {
		current := t.count.Load()
		if current >= t.max {
			return false
		}

		if t.count.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

// Release frees a connection slot
func (t *ConnectionTracker) Release() {
	for {
		current := t.count.Load()
		if current <= 0 {
			return
		}

		if t.count.CompareAndSwap(current, current-1) {
			return
		}
	}
}

// Count returns the current number of active connections
func (t *ConnectionTracker) Count() int {
	return int(t.count.Load())
}
