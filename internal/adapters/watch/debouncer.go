package watch

import (
	"sync"
	"time"
)

// debouncer coalesces rapid events into a single callback after a delay
type debouncer struct {
	duration time.Duration
	callback func(files []string)
	timer    *time.Timer
	files    map[string]struct{}
	mu       sync.Mutex
	stopped  bool
}

// newDebouncer creates a new debouncer with the specified duration and callback
func newDebouncer(duration time.Duration, callback func(files []string)) *debouncer {
	return &debouncer{
		duration: duration,
		callback: callback,
		files:    make(map[string]struct{}),
	}
}

// trigger registers a file change and resets the debounce timer
func (d *debouncer) trigger(file string) {
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

// stop stops the debouncer and cancels any pending callback
func (d *debouncer) stop() {
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
