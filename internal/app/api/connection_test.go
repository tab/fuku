package api

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ConnectionTracker_Acquire(t *testing.T) {
	tests := []struct {
		name    string
		max     int
		acquire int
		want    bool
	}{
		{
			name:    "within limit",
			max:     3,
			acquire: 1,
			want:    true,
		},
		{
			name:    "at limit",
			max:     2,
			acquire: 3,
			want:    false,
		},
		{
			name:    "single slot",
			max:     1,
			acquire: 1,
			want:    true,
		},
		{
			name:    "single slot exceeded",
			max:     1,
			acquire: 2,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewConnectionTracker(tt.max)

			var ok bool
			for range tt.acquire {
				ok = tracker.Acquire()
			}

			assert.Equal(t, tt.want, ok)
		})
	}
}

func Test_ConnectionTracker_Release(t *testing.T) {
	tracker := NewConnectionTracker(1)

	require.True(t, tracker.Acquire())
	require.False(t, tracker.Acquire())

	tracker.Release()

	assert.True(t, tracker.Acquire())
}

func Test_ConnectionTracker_DoubleRelease(t *testing.T) {
	tracker := NewConnectionTracker(1)

	require.True(t, tracker.Acquire())
	tracker.Release()
	tracker.Release()

	assert.Equal(t, 0, tracker.Count())

	require.True(t, tracker.Acquire())
	assert.False(t, tracker.Acquire())
}

func Test_ConnectionTracker_ReleaseWithoutAcquire(t *testing.T) {
	tracker := NewConnectionTracker(1)

	tracker.Release()

	assert.Equal(t, 0, tracker.Count())
}

func Test_ConnectionTracker_Count(t *testing.T) {
	tracker := NewConnectionTracker(5)

	assert.Equal(t, 0, tracker.Count())

	tracker.Acquire()
	tracker.Acquire()
	assert.Equal(t, 2, tracker.Count())

	tracker.Release()
	assert.Equal(t, 1, tracker.Count())
}

func Test_ConnectionTracker_Concurrent(t *testing.T) {
	tracker := NewConnectionTracker(10)

	var wg sync.WaitGroup

	for range 20 {
		wg.Go(func() {
			if tracker.Acquire() {
				defer tracker.Release()
			}
		})
	}

	wg.Wait()

	assert.Equal(t, 0, tracker.Count())
}
