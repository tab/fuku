package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_NewTimeline(t *testing.T) {
	tests := []struct {
		name         string
		capacity     int
		wantCapacity int
	}{
		{
			name:         "normal capacity",
			capacity:     20,
			wantCapacity: 20,
		},
		{
			name:         "zero capacity clamped to 1",
			capacity:     0,
			wantCapacity: 1,
		},
		{
			name:         "negative capacity clamped to 1",
			capacity:     -5,
			wantCapacity: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := NewTimeline(tt.capacity)

			assert.Equal(t, tt.wantCapacity, tl.capacity)
			assert.Equal(t, 0, tl.count)
			assert.Equal(t, 0, tl.index)
			assert.Len(t, tl.slots, tt.wantCapacity)
		})
	}
}

func Test_Timeline_Append(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		appends  []TimelineSlot
		want     []TimelineSlot
	}{
		{
			name:     "empty timeline returns all SlotEmpty",
			capacity: 5,
			appends:  nil,
			want:     []TimelineSlot{SlotEmpty, SlotEmpty, SlotEmpty, SlotEmpty, SlotEmpty},
		},
		{
			name:     "single append pads right with SlotEmpty",
			capacity: 5,
			appends:  []TimelineSlot{SlotRunning},
			want:     []TimelineSlot{SlotRunning, SlotEmpty, SlotEmpty, SlotEmpty, SlotEmpty},
		},
		{
			name:     "partial fill",
			capacity: 5,
			appends:  []TimelineSlot{SlotStarting, SlotRunning, SlotRunning},
			want:     []TimelineSlot{SlotStarting, SlotRunning, SlotRunning, SlotEmpty, SlotEmpty},
		},
		{
			name:     "full capacity",
			capacity: 5,
			appends:  []TimelineSlot{SlotStarting, SlotRunning, SlotRunning, SlotRunning, SlotFailed},
			want:     []TimelineSlot{SlotStarting, SlotRunning, SlotRunning, SlotRunning, SlotFailed},
		},
		{
			name:     "ring wraps and drops oldest",
			capacity: 5,
			appends:  []TimelineSlot{SlotStarting, SlotRunning, SlotRunning, SlotRunning, SlotFailed, SlotStopped},
			want:     []TimelineSlot{SlotRunning, SlotRunning, SlotRunning, SlotFailed, SlotStopped},
		},
		{
			name:     "multiple wraps maintain order",
			capacity: 3,
			appends:  []TimelineSlot{SlotStarting, SlotRunning, SlotFailed, SlotStopped, SlotStarting, SlotRunning, SlotRunning},
			want:     []TimelineSlot{SlotStarting, SlotRunning, SlotRunning},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := NewTimeline(tt.capacity)

			for _, s := range tt.appends {
				tl.Append(s)
			}

			assert.Equal(t, tt.want, tl.Slots())
		})
	}
}

func Test_Timeline_Count(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		appends  int
		want     int
	}{
		{
			name:     "empty timeline",
			capacity: 5,
			appends:  0,
			want:     0,
		},
		{
			name:     "partial fill",
			capacity: 5,
			appends:  3,
			want:     3,
		},
		{
			name:     "full capacity",
			capacity: 5,
			appends:  5,
			want:     5,
		},
		{
			name:     "past capacity caps at capacity",
			capacity: 5,
			appends:  8,
			want:     5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := NewTimeline(tt.capacity)

			for range tt.appends {
				tl.Append(SlotRunning)
			}

			assert.Equal(t, tt.want, tl.Count())
		})
	}
}

func Test_StatusToSlot(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   TimelineSlot
	}{
		{
			name:   "running maps to SlotRunning",
			status: StatusRunning,
			want:   SlotRunning,
		},
		{
			name:   "starting maps to SlotStarting",
			status: StatusStarting,
			want:   SlotStarting,
		},
		{
			name:   "restarting maps to SlotStarting",
			status: StatusRestarting,
			want:   SlotStarting,
		},
		{
			name:   "stopping maps to SlotStarting",
			status: StatusStopping,
			want:   SlotStarting,
		},
		{
			name:   "failed maps to SlotFailed",
			status: StatusFailed,
			want:   SlotFailed,
		},
		{
			name:   "stopped maps to SlotStopped",
			status: StatusStopped,
			want:   SlotStopped,
		},
		{
			name:   "unknown status maps to SlotEmpty",
			status: "unknown",
			want:   SlotEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StatusToSlot(tt.status))
		})
	}
}

func Test_Timeline_Backfill(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		n        int
		wantLen  int
	}{
		{
			name:     "backfill within capacity",
			capacity: 20,
			n:        5,
			wantLen:  5,
		},
		{
			name:     "backfill capped at capacity",
			capacity: 3,
			n:        10,
			wantLen:  3,
		},
		{
			name:     "backfill zero samples",
			capacity: 20,
			n:        0,
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := NewTimeline(tt.capacity)
			tl.Backfill(SlotStarting, tt.n)

			assert.Equal(t, tt.wantLen, tl.Count())

			slots := tl.Slots()
			for i := range tt.wantLen {
				assert.Equal(t, SlotStarting, slots[i])
			}
		})
	}
}

func Test_BackfillStartupHistory(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		startupSampled int
		startedAt      time.Time
		readyAt        time.Time
		wantAdded      int
	}{
		{
			name:           "multi-second with some amber already sampled",
			startupSampled: 2,
			startedAt:      t0,
			readyAt:        t0.Add(5 * time.Second),
			wantAdded:      3,
		},
		{
			name:           "sub-second with no amber sampled",
			startupSampled: 0,
			startedAt:      t0,
			readyAt:        t0.Add(200 * time.Millisecond),
			wantAdded:      1,
		},
		{
			name:           "sub-second with amber already sampled",
			startupSampled: 1,
			startedAt:      t0,
			readyAt:        t0.Add(200 * time.Millisecond),
			wantAdded:      0,
		},
		{
			name:           "exact match already sampled",
			startupSampled: 3,
			startedAt:      t0,
			readyAt:        t0.Add(3 * time.Second),
			wantAdded:      0,
		},
		{
			name:           "zero duration",
			startupSampled: 0,
			startedAt:      t0,
			readyAt:        t0,
			wantAdded:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := NewTimeline(20)
			svc := &ServiceState{
				Timeline:       tl,
				StartupSampled: tt.startupSampled,
			}

			backfillStartupHistory(svc, 1, tt.startedAt, tt.readyAt)

			assert.Equal(t, tt.wantAdded, tl.Count())
		})
	}
}

func Test_SampleTimelines_RestartingWithZeroStartTime(t *testing.T) {
	tl := NewTimeline(20)
	m := &Model{}
	m.state.services = map[string]*ServiceState{
		"svc": {
			Status:   StatusRestarting,
			Timeline: tl,
		},
	}

	m.sampleTimelines()

	assert.Equal(t, 1, tl.Count())
	assert.Equal(t, SlotStarting, tl.Slots()[0])
}

func Test_SampleTimelines_StartingWithZeroStartTimeSkipped(t *testing.T) {
	tl := NewTimeline(20)
	m := &Model{}
	m.state.services = map[string]*ServiceState{
		"svc": {
			Status:   StatusStarting,
			Timeline: tl,
		},
	}

	m.sampleTimelines()

	assert.Equal(t, 0, tl.Count())
}
