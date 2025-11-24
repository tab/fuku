package runner

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_AcquireRelease(t *testing.T) {
	pool := NewWorkerPool()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		err := pool.Acquire(ctx)
		require.NoError(t, err)
	}

	done := make(chan bool)

	go func() {
		err := pool.Acquire(ctx)
		require.NoError(t, err)

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
	ctx := context.Background()
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

			err := pool.Acquire(ctx)
			require.NoError(t, err)

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

func Test_AcquireContextCancelled(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool()

	for i := 0; i < 3; i++ {
		err := pool.Acquire(ctx)
		require.NoError(t, err)
	}

	ctx, cancel := context.WithCancel(ctx)

	done := make(chan error, 1)

	go func() {
		done <- pool.Acquire(ctx)
	}()

	time.Sleep(5 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Should have received context cancellation error")
	}

	for i := 0; i < 3; i++ {
		pool.Release()
	}
}
