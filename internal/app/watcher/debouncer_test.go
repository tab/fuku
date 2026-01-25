package watcher

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Debouncer_Trigger(t *testing.T) {
	var (
		mu            sync.Mutex
		called        int
		receivedFiles []string
	)

	d := NewDebouncer(50*time.Millisecond, func(files []string) {
		mu.Lock()
		defer mu.Unlock()

		called++
		receivedFiles = files
	})
	defer d.Stop()

	d.Trigger("file1.go")
	d.Trigger("file2.go")
	d.Trigger("file3.go")

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 1, called)
	assert.Len(t, receivedFiles, 3)
	mu.Unlock()
}

func Test_Debouncer_CoalescesRapidEvents(t *testing.T) {
	var (
		mu        sync.Mutex
		callCount int
	)

	d := NewDebouncer(50*time.Millisecond, func(files []string) {
		mu.Lock()
		defer mu.Unlock()

		callCount++
	})
	defer d.Stop()

	for i := 0; i < 10; i++ {
		d.Trigger("file.go")
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 1, callCount)
	mu.Unlock()
}

func Test_Debouncer_Stop(t *testing.T) {
	var called bool

	d := NewDebouncer(50*time.Millisecond, func(files []string) {
		called = true
	})

	d.Trigger("file.go")
	d.Stop()

	time.Sleep(100 * time.Millisecond)

	assert.False(t, called)
}

func Test_Debouncer_StopPreventsNewTriggers(t *testing.T) {
	var called bool

	d := NewDebouncer(50*time.Millisecond, func(files []string) {
		called = true
	})

	d.Stop()
	d.Trigger("file.go")

	time.Sleep(100 * time.Millisecond)

	assert.False(t, called)
}

func Test_Debouncer_MultipleCallbacks(t *testing.T) {
	var (
		mu        sync.Mutex
		callCount int
	)

	d := NewDebouncer(30*time.Millisecond, func(files []string) {
		mu.Lock()
		defer mu.Unlock()

		callCount++
	})
	defer d.Stop()

	d.Trigger("file1.go")
	time.Sleep(50 * time.Millisecond)

	d.Trigger("file2.go")
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 2, callCount)
	mu.Unlock()
}

func Test_Debouncer_UniqueFiles(t *testing.T) {
	var receivedFiles []string

	d := NewDebouncer(50*time.Millisecond, func(files []string) {
		receivedFiles = files
	})
	defer d.Stop()

	d.Trigger("file.go")
	d.Trigger("file.go")
	d.Trigger("file.go")

	time.Sleep(100 * time.Millisecond)

	assert.Len(t, receivedFiles, 1)
	assert.Equal(t, "file.go", receivedFiles[0])
}
