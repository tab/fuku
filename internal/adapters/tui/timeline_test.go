package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/model"
)

func Test_newTimeline(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := newTimeline(tt.capacity)

			assert.Equal(t, tt.wantCapacity, tl.capacity)
			assert.Equal(t, 0, tl.count)
			assert.Equal(t, 0, tl.index)
			assert.Len(t, tl.ring, tt.wantCapacity)
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
			name:     "empty timeline returns all TimelineSlotEmpty",
			capacity: 5,
			appends:  nil,
			want:     []TimelineSlot{TimelineSlotEmpty, TimelineSlotEmpty, TimelineSlotEmpty, TimelineSlotEmpty, TimelineSlotEmpty},
		},
		{
			name:     "single append pads right with TimelineSlotEmpty",
			capacity: 5,
			appends:  []TimelineSlot{TimelineSlotRunning},
			want:     []TimelineSlot{TimelineSlotRunning, TimelineSlotEmpty, TimelineSlotEmpty, TimelineSlotEmpty, TimelineSlotEmpty},
		},
		{
			name:     "partial fill",
			capacity: 5,
			appends:  []TimelineSlot{TimelineSlotStarting, TimelineSlotRunning, TimelineSlotRunning},
			want:     []TimelineSlot{TimelineSlotStarting, TimelineSlotRunning, TimelineSlotRunning, TimelineSlotEmpty, TimelineSlotEmpty},
		},
		{
			name:     "full capacity",
			capacity: 5,
			appends:  []TimelineSlot{TimelineSlotStarting, TimelineSlotRunning, TimelineSlotRunning, TimelineSlotRunning, TimelineSlotFailed},
			want:     []TimelineSlot{TimelineSlotStarting, TimelineSlotRunning, TimelineSlotRunning, TimelineSlotRunning, TimelineSlotFailed},
		},
		{
			name:     "ring wraps and drops oldest",
			capacity: 5,
			appends:  []TimelineSlot{TimelineSlotStarting, TimelineSlotRunning, TimelineSlotRunning, TimelineSlotRunning, TimelineSlotFailed, TimelineSlotStopped},
			want:     []TimelineSlot{TimelineSlotRunning, TimelineSlotRunning, TimelineSlotRunning, TimelineSlotFailed, TimelineSlotStopped},
		},
		{
			name:     "multiple wraps maintain order",
			capacity: 3,
			appends:  []TimelineSlot{TimelineSlotStarting, TimelineSlotRunning, TimelineSlotFailed, TimelineSlotStopped, TimelineSlotStarting, TimelineSlotRunning, TimelineSlotRunning},
			want:     []TimelineSlot{TimelineSlotStarting, TimelineSlotRunning, TimelineSlotRunning},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := newTimeline(tt.capacity)

			for _, s := range tt.appends {
				tl.Append(s)
			}

			assert.Equal(t, tt.want, tl.slots())
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
			tl := newTimeline(tt.capacity)

			for range tt.appends {
				tl.Append(TimelineSlotRunning)
			}

			assert.Equal(t, tt.want, tl.Count())
		})
	}
}

func Test_statusToSlot(t *testing.T) {
	tests := []struct {
		name   string
		status model.Status
		want   TimelineSlot
	}{
		{
			name:   "running maps to TimelineSlotRunning",
			status: model.StatusRunning,
			want:   TimelineSlotRunning,
		},
		{
			name:   "starting maps to TimelineSlotStarting",
			status: model.StatusStarting,
			want:   TimelineSlotStarting,
		},
		{
			name:   "restarting maps to TimelineSlotStarting",
			status: model.StatusRestarting,
			want:   TimelineSlotStarting,
		},
		{
			name:   "stopping maps to TimelineSlotStarting",
			status: model.StatusStopping,
			want:   TimelineSlotStarting,
		},
		{
			name:   "failed maps to TimelineSlotFailed",
			status: model.StatusFailed,
			want:   TimelineSlotFailed,
		},
		{
			name:   "stopped maps to TimelineSlotStopped",
			status: model.StatusStopped,
			want:   TimelineSlotStopped,
		},
		{
			name:   "pending maps to TimelineSlotEmpty",
			status: model.StatusPending,
			want:   TimelineSlotEmpty,
		},
		{
			name:   "unknown status maps to TimelineSlotEmpty",
			status: "unknown",
			want:   TimelineSlotEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusToSlot(tt.status))
		})
	}
}

func Test_Timeline_backfill(t *testing.T) {
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
			tl := newTimeline(tt.capacity)
			tl.backfill(tt.n)

			assert.Equal(t, tt.wantLen, tl.Count())

			slots := tl.slots()
			for i := range tt.wantLen {
				assert.Equal(t, TimelineSlotStarting, slots[i])
			}
		})
	}
}

func Test_Timeline_backfill_EvictsOldSamplesOnFullStrip(t *testing.T) {
	tl := newTimeline(5)
	for range 5 {
		tl.Append(TimelineSlotRunning)
	}

	tl.backfill(2)

	assert.Equal(t, 5, tl.Count())

	slots := tl.slots()
	assert.Equal(t, TimelineSlotRunning, slots[0])
	assert.Equal(t, TimelineSlotRunning, slots[1])
	assert.Equal(t, TimelineSlotRunning, slots[2])
	assert.Equal(t, TimelineSlotStarting, slots[3])
	assert.Equal(t, TimelineSlotStarting, slots[4])
}

func Test_BackfillStartupHistory_FullStrip(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tl := newTimeline(5)
	for range 5 {
		tl.Append(TimelineSlotRunning)
	}

	view := &serviceView{
		Timeline: tl,
	}

	backfillStartupHistory(view, t0, t0.Add(3*time.Second))

	assert.Equal(t, 5, tl.Count())

	slots := tl.slots()
	assert.Equal(t, TimelineSlotRunning, slots[0])
	assert.Equal(t, TimelineSlotRunning, slots[1])
	assert.Equal(t, TimelineSlotStarting, slots[2])
	assert.Equal(t, TimelineSlotStarting, slots[3])
	assert.Equal(t, TimelineSlotStarting, slots[4])
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
			tl := newTimeline(20)
			view := &serviceView{
				Timeline:       tl,
				StartupSampled: tt.startupSampled,
			}

			backfillStartupHistory(view, tt.startedAt, tt.readyAt)

			assert.Equal(t, tt.wantAdded, tl.Count())
		})
	}
}

func Test_SampleTimelines_RestartingWithZeroStartedAt(t *testing.T) {
	tl := newTimeline(20)
	m := &Model{snapshot: &model.Snapshot{Services: map[string]*model.Service{"svc": {Status: model.StatusRestarting}}}}
	m.state.views = map[string]*serviceView{
		"svc": {
			Timeline: tl,
		},
	}

	m.sampleTimelines()

	assert.Equal(t, 1, tl.Count())
	assert.Equal(t, TimelineSlotStarting, tl.slots()[0])
}

func Test_SampleTimelines_StartingWithZeroStartedAtSkipped(t *testing.T) {
	tl := newTimeline(20)
	m := &Model{snapshot: &model.Snapshot{Services: map[string]*model.Service{"svc": {Status: model.StatusStarting}}}}
	m.state.views = map[string]*serviceView{
		"svc": {
			Timeline: tl,
		},
	}

	m.sampleTimelines()

	assert.Equal(t, 0, tl.Count())
}
