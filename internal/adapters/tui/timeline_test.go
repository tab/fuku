package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
)

func Test_newTimeline(t *testing.T) {
	tl := newTimeline()

	assert.Equal(t, terminal.TimelineDefaultSlots, tl.capacity)
	assert.Equal(t, 0, tl.count)
	assert.Equal(t, 0, tl.index)
	assert.Len(t, tl.ring, terminal.TimelineDefaultSlots)
}

func Test_Timeline_Append(t *testing.T) {
	tests := []struct {
		name    string
		appends []TimelineSlot
		want    []TimelineSlot
	}{
		{
			name:    "empty timeline returns all TimelineSlotEmpty",
			appends: nil,
			want:    make([]TimelineSlot, terminal.TimelineDefaultSlots),
		},
		{
			name:    "single append pads right with TimelineSlotEmpty",
			appends: []TimelineSlot{TimelineSlotRunning},
			want:    append([]TimelineSlot{TimelineSlotRunning}, make([]TimelineSlot, terminal.TimelineDefaultSlots-1)...),
		},
		{
			name:    "partial fill",
			appends: []TimelineSlot{TimelineSlotStarting, TimelineSlotRunning, TimelineSlotRunning},
			want:    append([]TimelineSlot{TimelineSlotStarting, TimelineSlotRunning, TimelineSlotRunning}, make([]TimelineSlot, terminal.TimelineDefaultSlots-3)...),
		},
		{
			name:    "full capacity",
			appends: append(slices.Repeat([]TimelineSlot{TimelineSlotRunning}, terminal.TimelineDefaultSlots-1), TimelineSlotFailed),
			want:    append(slices.Repeat([]TimelineSlot{TimelineSlotRunning}, terminal.TimelineDefaultSlots-1), TimelineSlotFailed),
		},
		{
			name:    "ring wraps and drops oldest",
			appends: append(slices.Repeat([]TimelineSlot{TimelineSlotRunning}, terminal.TimelineDefaultSlots), TimelineSlotFailed),
			want:    append(slices.Repeat([]TimelineSlot{TimelineSlotRunning}, terminal.TimelineDefaultSlots-1), TimelineSlotFailed),
		},
		{
			name: "multiple wraps maintain order",
			appends: append(
				slices.Repeat([]TimelineSlot{TimelineSlotRunning}, 2*terminal.TimelineDefaultSlots),
				TimelineSlotStarting, TimelineSlotFailed, TimelineSlotStopped,
			),
			want: append(
				slices.Repeat([]TimelineSlot{TimelineSlotRunning}, terminal.TimelineDefaultSlots-3),
				TimelineSlotStarting, TimelineSlotFailed, TimelineSlotStopped,
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := newTimeline()

			for _, s := range tt.appends {
				tl.Append(s)
			}

			assert.Equal(t, tt.want, tl.slots())
		})
	}
}

func Test_Timeline_Count(t *testing.T) {
	tests := []struct {
		name    string
		appends int
		want    int
	}{
		{
			name:    "empty timeline",
			appends: 0,
			want:    0,
		},
		{
			name:    "partial fill",
			appends: 3,
			want:    3,
		},
		{
			name:    "full capacity",
			appends: terminal.TimelineDefaultSlots,
			want:    terminal.TimelineDefaultSlots,
		},
		{
			name:    "past capacity caps at capacity",
			appends: terminal.TimelineDefaultSlots + 3,
			want:    terminal.TimelineDefaultSlots,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := newTimeline()

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
		name    string
		n       int
		wantLen int
	}{
		{
			name:    "backfill within capacity",
			n:       5,
			wantLen: 5,
		},
		{
			name:    "backfill capped at capacity",
			n:       terminal.TimelineDefaultSlots + 4,
			wantLen: terminal.TimelineDefaultSlots,
		},
		{
			name:    "backfill zero samples",
			n:       0,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := newTimeline()
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
	tl := newTimeline()
	for range terminal.TimelineDefaultSlots {
		tl.Append(TimelineSlotRunning)
	}

	tl.backfill(2)

	assert.Equal(t, terminal.TimelineDefaultSlots, tl.Count())

	slots := tl.slots()
	for i := range terminal.TimelineDefaultSlots - 2 {
		assert.Equal(t, TimelineSlotRunning, slots[i])
	}

	assert.Equal(t, TimelineSlotStarting, slots[terminal.TimelineDefaultSlots-2])
	assert.Equal(t, TimelineSlotStarting, slots[terminal.TimelineDefaultSlots-1])
}

func Test_BackfillStartupHistory_FullStrip(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tl := newTimeline()
	for range terminal.TimelineDefaultSlots {
		tl.Append(TimelineSlotRunning)
	}

	view := &serviceView{
		Timeline: tl,
	}

	backfillStartupHistory(view, t0, t0.Add(3*time.Second))

	assert.Equal(t, terminal.TimelineDefaultSlots, tl.Count())

	slots := tl.slots()
	for i := range terminal.TimelineDefaultSlots - 3 {
		assert.Equal(t, TimelineSlotRunning, slots[i])
	}

	assert.Equal(t, TimelineSlotStarting, slots[terminal.TimelineDefaultSlots-3])
	assert.Equal(t, TimelineSlotStarting, slots[terminal.TimelineDefaultSlots-2])
	assert.Equal(t, TimelineSlotStarting, slots[terminal.TimelineDefaultSlots-1])
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
			tl := newTimeline()
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
	tl := newTimeline()
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
	tl := newTimeline()
	m := &Model{snapshot: &model.Snapshot{Services: map[string]*model.Service{"svc": {Status: model.StatusStarting}}}}
	m.state.views = map[string]*serviceView{
		"svc": {
			Timeline: tl,
		},
	}

	m.sampleTimelines()

	assert.Equal(t, 0, tl.Count())
}
