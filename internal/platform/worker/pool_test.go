package worker

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewPool(t *testing.T) {
	pool := NewPool(Options{Workers: 3})

	assert.Equal(t, 3, cap(pool.sem))
}

func Test_Pool_Acquire(t *testing.T) {
	tests := []struct {
		name     string
		unblock  func(pool *Pool, cancel context.CancelFunc)
		expected error
	}{
		{
			name:    "waits for a release when every slot is taken",
			unblock: func(pool *Pool, _ context.CancelFunc) { pool.Release() },
		},
		{
			name:     "returns the context error when cancelled while waiting",
			unblock:  func(_ *Pool, cancel context.CancelFunc) { cancel() },
			expected: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				pool := NewPool(Options{Workers: 2})

				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				for range 2 {
					require.NoError(t, pool.Acquire(ctx))
				}

				done := make(chan error, 1)

				go func() { done <- pool.Acquire(ctx) }()

				synctest.Wait()

				assert.Empty(t, done)

				tt.unblock(pool, cancel)

				synctest.Wait()

				require.ErrorIs(t, <-done, tt.expected)
			})
		})
	}
}

func Test_Pool_Acquire_BoundsConcurrency(t *testing.T) {
	options := Options{Workers: 5}
	pool := NewPool(options)

	var (
		activeWorkers int
		maxActive     int
		mu            sync.Mutex
	)

	workersStarted := make(chan struct{}, 10)
	workersCanFinish := make(chan struct{})

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			//nolint:testifylint // require calls FailNow, which a goroutine must not
			assert.NoError(t, pool.Acquire(t.Context()))

			defer pool.Release()

			mu.Lock()

			activeWorkers++
			maxActive = max(maxActive, activeWorkers)

			mu.Unlock()

			workersStarted <- struct{}{}

			<-workersCanFinish

			mu.Lock()

			activeWorkers--

			mu.Unlock()
		})
	}

	for range options.Workers {
		<-workersStarted
	}

	close(workersCanFinish)
	wg.Wait()

	assert.Equal(t, 0, activeWorkers)
	assert.LessOrEqual(t, maxActive, options.Workers)
	assert.Positive(t, maxActive)
}
