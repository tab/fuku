package watcher

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Debouncer_Trigger(t *testing.T) {
	var (
		mu            sync.Mutex
		receivedFiles []string
	)

	done := make(chan struct{})

	d := NewDebouncer(10*time.Millisecond, func(files []string) {
		mu.Lock()

		receivedFiles = files

		mu.Unlock()
		close(done)
	})
	defer d.Stop()

	d.Trigger("file1.go")
	d.Trigger("file2.go")
	d.Trigger("file3.go")

	select {
	case <-done:
		mu.Lock()
		assert.Len(t, receivedFiles, 3)
		mu.Unlock()
	case <-time.After(time.Second):
		t.Fatal("debouncer callback was not called")
	}
}

func Test_Debouncer_CoalescesRapidEvents(t *testing.T) {
	var (
		mu        sync.Mutex
		callCount int
	)

	done := make(chan struct{}, 10)

	d := NewDebouncer(50*time.Millisecond, func(files []string) {
		mu.Lock()

		callCount++

		mu.Unlock()

		done <- struct{}{}
	})
	defer d.Stop()

	// Trigger rapidly - each trigger resets the 50ms timer
	// Total time: 10 * 10ms = 100ms of triggering, then 50ms debounce = ~150ms
	for i := 0; i < 10; i++ {
		d.Trigger("file.go")
		time.Sleep(10 * time.Millisecond) //nolint:forbidigo // intentional - testing debounce coalescing
	}

	select {
	case <-done:
		mu.Lock()
		assert.Equal(t, 1, callCount, "should coalesce into single callback")
		mu.Unlock()
	case <-time.After(time.Second):
		t.Fatal("debouncer callback was not called")
	}
}

func Test_Debouncer_Stop(t *testing.T) {
	called := make(chan struct{})

	d := NewDebouncer(10*time.Millisecond, func(files []string) {
		close(called)
	})

	d.Trigger("file.go")
	d.Stop()

	select {
	case <-called:
		t.Fatal("callback should not be called after Stop")
	case <-time.After(50 * time.Millisecond):
		// Expected: callback not called
	}
}

func Test_Debouncer_StopPreventsNewTriggers(t *testing.T) {
	called := make(chan struct{})

	d := NewDebouncer(10*time.Millisecond, func(files []string) {
		close(called)
	})

	d.Stop()
	d.Trigger("file.go")

	select {
	case <-called:
		t.Fatal("callback should not be called after Stop")
	case <-time.After(50 * time.Millisecond):
		// Expected: callback not called
	}
}

func Test_Debouncer_MultipleCallbacks(t *testing.T) {
	var (
		mu        sync.Mutex
		callCount int
	)

	done := make(chan struct{}, 10)

	d := NewDebouncer(10*time.Millisecond, func(files []string) {
		mu.Lock()

		callCount++

		mu.Unlock()

		done <- struct{}{}
	})
	defer d.Stop()

	// First batch
	d.Trigger("file1.go")

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("first callback was not called")
	}

	// Second batch
	d.Trigger("file2.go")

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second callback was not called")
	}

	mu.Lock()
	assert.Equal(t, 2, callCount)
	mu.Unlock()
}

func Test_Debouncer_UniqueFiles(t *testing.T) {
	var (
		mu            sync.Mutex
		receivedFiles []string
	)

	done := make(chan struct{})

	d := NewDebouncer(10*time.Millisecond, func(files []string) {
		mu.Lock()

		receivedFiles = files

		mu.Unlock()
		close(done)
	})
	defer d.Stop()

	d.Trigger("file.go")
	d.Trigger("file.go")
	d.Trigger("file.go")

	select {
	case <-done:
		mu.Lock()
		require.Len(t, receivedFiles, 1)
		assert.Equal(t, "file.go", receivedFiles[0])
		mu.Unlock()
	case <-time.After(time.Second):
		t.Fatal("debouncer callback was not called")
	}
}
