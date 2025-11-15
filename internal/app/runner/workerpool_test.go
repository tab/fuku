package runner

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_AcquireRelease(t *testing.T) {
	pool := NewWorkerPool()

	for i := 0; i < 3; i++ {
		pool.Acquire()
	}

	done := make(chan bool)

	go func() {
		pool.Acquire()

		done <- true
	}()

	select {
	case <-done:
		t.Fatal("Should not have acquired fourth worker slot immediately")
	case <-time.After(50 * time.Millisecond):
	}

	pool.Release()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Should have acquired worker slot after release")
	}

	for i := 0; i < 3; i++ {
		pool.Release()
	}
}

func Test_ConcurrentWorkers(t *testing.T) {
	pool := NewWorkerPool()

	var (
		activeWorkers int
		maxActive     int
		mu            sync.Mutex
	)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			pool.Acquire()
			defer pool.Release()

			mu.Lock()

			activeWorkers++
			if activeWorkers > maxActive {
				maxActive = activeWorkers
			}

			mu.Unlock()

			time.Sleep(10 * time.Millisecond)

			mu.Lock()

			activeWorkers--

			mu.Unlock()
		}()
	}

	wg.Wait()

	assert.Equal(t, 0, activeWorkers)
	assert.LessOrEqual(t, maxActive, 3)
	assert.Greater(t, maxActive, 0)
}
