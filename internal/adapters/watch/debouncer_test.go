package watch

import (
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_debouncer_trigger(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var (
			mu            sync.Mutex
			receivedFiles []string
		)

		done := make(chan struct{})

		d := newDebouncer(10*time.Millisecond, func(files []string) {
			mu.Lock()

			receivedFiles = files

			mu.Unlock()
			close(done)
		})
		defer d.stop()

		d.trigger("file1.go")
		d.trigger("file2.go")
		d.trigger("file3.go")

		select {
		case <-done:
			mu.Lock()
			assert.Len(t, receivedFiles, 3)
			mu.Unlock()
		case <-time.After(time.Second):
			t.Fatal("debouncer callback was not called")
		}
	})
}

func Test_debouncer_CoalescesRapidEvents(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var (
			mu        sync.Mutex
			callCount int
		)

		done := make(chan struct{}, 10)

		d := newDebouncer(50*time.Millisecond, func(files []string) {
			mu.Lock()

			callCount++

			mu.Unlock()

			done <- struct{}{}
		})
		defer d.stop()

		for range 10 {
			d.trigger("file.go")
			<-time.After(10 * time.Millisecond)
		}

		select {
		case <-done:
			mu.Lock()
			assert.Equal(t, 1, callCount, "should coalesce into single callback")
			mu.Unlock()
		case <-time.After(time.Second):
			t.Fatal("debouncer callback was not called")
		}
	})
}

func Test_debouncer_stop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		called := make(chan struct{})

		d := newDebouncer(10*time.Millisecond, func(files []string) {
			close(called)
		})

		d.trigger("file.go")
		d.stop()

		select {
		case <-called:
			t.Fatal("callback should not be called after Stop")
		case <-time.After(50 * time.Millisecond):
		}
	})
}

func Test_debouncer_stopPreventsNewTriggers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		called := make(chan struct{})

		d := newDebouncer(10*time.Millisecond, func(files []string) {
			close(called)
		})

		d.stop()
		d.trigger("file.go")

		select {
		case <-called:
			t.Fatal("callback should not be called after Stop")
		case <-time.After(50 * time.Millisecond):
		}
	})
}

func Test_debouncer_MultipleCallbacks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var (
			mu        sync.Mutex
			callCount int
		)

		done := make(chan struct{}, 10)

		d := newDebouncer(10*time.Millisecond, func(files []string) {
			mu.Lock()

			callCount++

			mu.Unlock()

			done <- struct{}{}
		})
		defer d.stop()

		d.trigger("file1.go")

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("first callback was not called")
		}

		d.trigger("file2.go")

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("second callback was not called")
		}

		mu.Lock()
		assert.Equal(t, 2, callCount)
		mu.Unlock()
	})
}

func Test_debouncer_UniqueFiles(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var (
			mu            sync.Mutex
			receivedFiles []string
		)

		done := make(chan struct{})

		d := newDebouncer(10*time.Millisecond, func(files []string) {
			mu.Lock()

			receivedFiles = files

			mu.Unlock()
			close(done)
		})
		defer d.stop()

		d.trigger("file.go")
		d.trigger("file.go")
		d.trigger("file.go")

		select {
		case <-done:
			mu.Lock()
			require.Len(t, receivedFiles, 1)
			assert.Equal(t, "file.go", receivedFiles[0])
			mu.Unlock()
		case <-time.After(time.Second):
			t.Fatal("debouncer callback was not called")
		}
	})
}

func Test_debouncer_fire(t *testing.T) {
	var batches [][]string

	record := func(files []string) {
		batches = append(batches, files)
	}

	tests := []struct {
		name     string
		before   func() *debouncer
		expected [][]string
	}{
		{
			name: "fires the pending batch",
			before: func() *debouncer {
				batches = nil

				d := newDebouncer(time.Hour, record)
				d.trigger("main.go")

				return d
			},
			expected: [][]string{{"main.go"}},
		},
		{
			name: "an empty batch fires nothing",
			before: func() *debouncer {
				batches = nil

				return newDebouncer(time.Hour, record)
			},
			expected: nil,
		},
		{
			name: "a stopped debouncer fires nothing",
			before: func() *debouncer {
				batches = nil

				d := newDebouncer(time.Hour, record)
				d.trigger("main.go")
				d.stop()

				return d
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.before()
			defer d.stop()

			d.fire()

			assert.Equal(t, tt.expected, batches)
		})
	}
}
