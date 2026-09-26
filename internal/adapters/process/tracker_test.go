package process

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_NewTracker(t *testing.T) {
	tracker := NewTracker()

	assert.NotNil(t, tracker)
	assert.NotNil(t, tracker.active)
	assert.NotNil(t, tracker.detached)
}

func Test_Tracker_track(t *testing.T) {
	handle := &Handle{svc: model.Service{ID: "test-id-api", Name: "api"}, done: make(chan struct{})}
	startErr := errors.New("start failed")

	tests := []struct {
		name     string
		start    func() (*Handle, error)
		expected *Handle
		err      error
		tracked  bool
	}{
		{
			name: "a started child is tracked under its service ID",
			start: func() (*Handle, error) {
				return handle, nil
			},
			expected: handle,
			tracked:  true,
		},
		{
			name: "a failed start tracks nothing",
			start: func() (*Handle, error) {
				return nil, startErr
			},
			err: startErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewTracker()

			result, err := tracker.track(tt.start)

			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.expected, result)

			_, exists := tracker.Get("test-id-api")
			assert.Equal(t, tt.tracked, exists)
		})
	}
}

func Test_Tracker_Get(t *testing.T) {
	tracker := NewTracker()

	active := &Handle{svc: model.Service{ID: "test-id-api", Name: "api"}, done: make(chan struct{})}
	detached := &Handle{svc: model.Service{ID: "test-id-web", Name: "web"}, done: make(chan struct{})}
	starting := func(handle *Handle) func() (*Handle, error) {
		return func() (*Handle, error) { return handle, nil }
	}

	_, err := tracker.track(starting(active))
	require.NoError(t, err)

	_, err = tracker.track(starting(detached))
	require.NoError(t, err)

	tracker.Detach("test-id-web")

	tests := []struct {
		name     string
		id       string
		expected contracts.Process
		exists   bool
	}{
		{
			name:     "an active child is found",
			id:       "test-id-api",
			expected: active,
			exists:   true,
		},
		{
			name:     "a detached child is still found",
			id:       "test-id-web",
			expected: detached,
			exists:   true,
		},
		{
			name: "an unknown service has no child",
			id:   "test-id-db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc, exists := tracker.Get(tt.id)

			assert.Equal(t, tt.exists, exists)
			assert.Equal(t, tt.expected, proc)
		})
	}
}

func Test_Tracker_Untrack(t *testing.T) {
	svc := model.Service{ID: "test-id-api", Name: "api"}
	starting := func(handle *Handle) func() (*Handle, error) {
		return func() (*Handle, error) { return handle, nil }
	}

	tests := []struct {
		name       string
		before     func(tracker *Tracker) (*Handle, contracts.Process)
		unexpected bool
	}{
		{
			name: "an active child is forgotten as an unexpected exit",
			before: func(tracker *Tracker) (*Handle, contracts.Process) {
				handle := &Handle{svc: svc, done: make(chan struct{})}
				_, _ = tracker.track(starting(handle))

				return handle, nil
			},
			unexpected: true,
		},
		{
			name: "a detached child is forgotten quietly",
			before: func(tracker *Tracker) (*Handle, contracts.Process) {
				handle := &Handle{svc: svc, done: make(chan struct{})}
				_, _ = tracker.track(starting(handle))
				tracker.Detach(svc.ID)

				return handle, nil
			},
		},
		{
			name: "an unknown child changes nothing",
			before: func(_ *Tracker) (*Handle, contracts.Process) {
				return &Handle{svc: svc, done: make(chan struct{})}, nil
			},
		},
		{
			name: "another handle for the same service is kept",
			before: func(tracker *Tracker) (*Handle, contracts.Process) {
				tracked := &Handle{svc: svc, done: make(chan struct{})}
				_, _ = tracker.track(starting(tracked))

				return &Handle{svc: svc, done: make(chan struct{})}, tracked
			},
		},
		{
			name: "a replaced child cannot untrack its successor",
			before: func(tracker *Tracker) (*Handle, contracts.Process) {
				old := &Handle{svc: svc, done: make(chan struct{})}
				_, _ = tracker.track(starting(old))
				tracker.Detach(svc.ID)

				successor := &Handle{svc: svc, done: make(chan struct{})}
				_, _ = tracker.track(starting(successor))

				return old, successor
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewTracker()
			handle, remaining := tt.before(tracker)

			unexpected := tracker.Untrack(svc.ID, handle)

			assert.Equal(t, tt.unexpected, unexpected)

			proc, exists := tracker.Get(svc.ID)
			assert.Equal(t, remaining != nil, exists)
			assert.Equal(t, remaining, proc)
		})
	}
}

func Test_Tracker_Detach_Unknown(t *testing.T) {
	tracker := NewTracker()

	tracker.Detach("test-id-api")

	_, exists := tracker.Get("test-id-api")
	assert.False(t, exists)
}

func Test_Tracker_Reverse(t *testing.T) {
	tracker := NewTracker()

	first := &Handle{svc: model.Service{ID: "test-id-db", Name: "db"}, done: make(chan struct{})}
	second := &Handle{svc: model.Service{ID: "test-id-api", Name: "api"}, done: make(chan struct{})}
	third := &Handle{svc: model.Service{ID: "test-id-web", Name: "web"}, done: make(chan struct{})}
	starting := func(handle *Handle) func() (*Handle, error) {
		return func() (*Handle, error) { return handle, nil }
	}

	for _, handle := range []*Handle{first, second, third} {
		_, err := tracker.track(starting(handle))
		require.NoError(t, err)
	}

	tracker.Detach("test-id-db")

	procs := tracker.Reverse()

	assert.Equal(t, []contracts.Process{third, second, first}, procs)
}

func Test_Tracker_ConcurrentAccess(t *testing.T) {
	tracker := NewTracker()

	count := 10
	handles := make([]*Handle, count)
	starting := func(handle *Handle) func() (*Handle, error) {
		return func() (*Handle, error) { return handle, nil }
	}

	var trackWg sync.WaitGroup

	for i := range count {
		id := fmt.Sprintf("test-id-%d", i)
		handles[i] = &Handle{svc: model.Service{ID: id, Name: id}, done: make(chan struct{})}

		trackWg.Add(1)

		go func(handle *Handle) {
			defer trackWg.Done()

			_, _ = tracker.track(starting(handle))
		}(handles[i])
	}

	trackWg.Wait()

	var accessWg sync.WaitGroup

	for range 5 {
		accessWg.Add(2)

		go func() {
			defer accessWg.Done()

			_, _ = tracker.Get("test-id-0")
		}()

		go func() {
			defer accessWg.Done()

			tracker.Reverse()
		}()
	}

	accessWg.Wait()

	for i, handle := range handles {
		tracker.Untrack(fmt.Sprintf("test-id-%d", i), handle)
	}

	assert.Empty(t, tracker.Reverse())
}
