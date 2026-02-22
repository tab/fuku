package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/config"
)

func Test_AcquireRelease(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	worker := NewWorkerPool(cfg)

	for i := 0; i < cfg.Concurrency.Workers; i++ {
		err := worker.Acquire(ctx)
		require.NoError(t, err)
	}

	done := make(chan bool)

	go func() {
		err := worker.Acquire(ctx)
		require.NoError(t, err)

		done <- true
	}()

	select {
	case <-done:
		t.Fatal("Should not have acquired extra worker slot immediately")
	case <-time.After(50 * time.Millisecond):
	}

	worker.Release()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Should have acquired worker slot after release")
	}

	for i := 0; i < cfg.Concurrency.Workers; i++ {
		worker.Release()
	}
}

func Test_ConcurrentWorkers(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	worker := NewWorkerPool(cfg)

	var (
		activeWorkers int
		maxActive     int
		mu            sync.Mutex
	)

	workersStarted := make(chan struct{}, 10)
	workersCanFinish := make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			err := worker.Acquire(ctx)
			require.NoError(t, err)

			defer worker.Release()

			mu.Lock()

			activeWorkers++
			if activeWorkers > maxActive {
				maxActive = activeWorkers
			}

			mu.Unlock()

			workersStarted <- struct{}{}

			<-workersCanFinish

			mu.Lock()

			activeWorkers--

			mu.Unlock()
		}()
	}

	for i := 0; i < cfg.Concurrency.Workers; i++ {
		<-workersStarted
	}

	close(workersCanFinish)
	wg.Wait()

	assert.Equal(t, 0, activeWorkers)
	assert.LessOrEqual(t, maxActive, cfg.Concurrency.Workers)
	assert.Greater(t, maxActive, 0)
}

func Test_AcquireContextCancelled(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	worker := NewWorkerPool(cfg)

	for i := 0; i < cfg.Concurrency.Workers; i++ {
		err := worker.Acquire(ctx)
		require.NoError(t, err)
	}

	ctx, cancel := context.WithCancel(ctx)

	done := make(chan error, 1)

	go func() {
		done <- worker.Acquire(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Should have received context cancellation error")
	}

	for i := 0; i < cfg.Concurrency.Workers; i++ {
		worker.Release()
	}
}
