package watcher

import (
	"sync"
	"time"
)

// Debouncer coalesces rapid events into a single callback after a delay
type Debouncer interface {
	Trigger(file string)
	Stop()
}

// debouncer implements the Debouncer interface
type debouncer struct {
	duration time.Duration
	callback func(files []string)
	timer    *time.Timer
	files    map[string]struct{}
	mu       sync.Mutex
	stopped  bool
}

// NewDebouncer creates a new Debouncer with the specified duration and callback
func NewDebouncer(duration time.Duration, callback func(files []string)) Debouncer {
	return &debouncer{
		duration: duration,
		callback: callback,
		files:    make(map[string]struct{}),
	}
}

// Trigger registers a file change and resets the debounce timer
func (d *debouncer) Trigger(file string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}

	d.files[file] = struct{}{}

	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(d.duration, d.fire)
}

// Stop stops the debouncer and cancels any pending callback
func (d *debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stopped = true

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}

	d.files = make(map[string]struct{})
}

// fire executes the callback with accumulated files
func (d *debouncer) fire() {
	d.mu.Lock()

	if d.stopped || len(d.files) == 0 {
		d.mu.Unlock()
		return
	}

	files := make([]string, 0, len(d.files))
	for f := range d.files {
		files = append(files, f)
	}

	d.files = make(map[string]struct{})
	d.timer = nil

	d.mu.Unlock()

	d.callback(files)
}
