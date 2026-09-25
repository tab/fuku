package tui

import (
	"time"

	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
)

// TimelineSlot represents the state of a single timeline sample
type TimelineSlot uint8

// Timeline slot constants
const (
	TimelineSlotEmpty TimelineSlot = iota
	TimelineSlotRunning
	TimelineSlotStarting
	TimelineSlotFailed
	TimelineSlotStopped
)

// Timeline is a fixed-capacity ring buffer that records per-second service state samples
type Timeline struct {
	capacity int
	ring     []TimelineSlot
	index    int
	count    int
}

// newTimeline creates a new Timeline at the default capacity
func newTimeline() *Timeline {
	return &Timeline{
		capacity: terminal.TimelineDefaultSlots,
		ring:     make([]TimelineSlot, terminal.TimelineDefaultSlots),
	}
}

// Append adds a sample to the timeline, dropping the oldest when full
func (t *Timeline) Append(slot TimelineSlot) {
	t.ring[t.index] = slot
	t.index = (t.index + 1) % t.capacity

	if t.count < t.capacity {
		t.count++
	}
}

// Count returns the number of observed samples
func (t *Timeline) Count() int {
	return t.count
}

// slots returns the current window in chronological order, padded on the right with TimelineSlotEmpty
func (t *Timeline) slots() []TimelineSlot {
	result := make([]TimelineSlot, t.capacity)

	if t.count == 0 {
		return result
	}

	start := 0
	if t.count == t.capacity {
		start = t.index
	}

	for i := range t.count {
		result[i] = t.ring[(start+i)%t.capacity]
	}

	return result
}

// backfill appends n starting samples, capped by timeline capacity
func (t *Timeline) backfill(n int) {
	for range min(n, t.capacity) {
		t.Append(TimelineSlotStarting)
	}
}

// backfillStartupHistory seeds the amber slots an attempt that ran from startedAt to settledAt is missing
func backfillStartupHistory(view *serviceView, startedAt time.Time, settledAt time.Time) {
	if startedAt.IsZero() || !settledAt.After(startedAt) {
		return
	}

	desired := max(1, int(settledAt.Sub(startedAt).Seconds()))
	missing := max(0, desired-view.StartupSampled)

	view.Timeline.backfill(missing)
	view.StartupSampled = max(view.StartupSampled, desired)
}

// sampleTimelines appends the current status of each service to its timeline
func (m *Model) sampleTimelines() {
	for id, view := range m.state.views {
		service := m.snapshot.Services[id]

		if service.Process.StartedAt.IsZero() && view.Timeline.Count() == 0 &&
			service.Status != model.StatusFailed &&
			service.Status != model.StatusStopped &&
			service.Status != model.StatusRestarting {
			continue
		}

		slot := statusToSlot(service.Status)
		view.Timeline.Append(slot)

		if slot == TimelineSlotStarting {
			view.StartupSampled++
		}
	}
}

// statusToSlot maps a service Status to the corresponding TimelineSlot
func statusToSlot(status model.Status) TimelineSlot {
	switch status {
	case model.StatusRunning:
		return TimelineSlotRunning
	case model.StatusStarting, model.StatusRestarting, model.StatusStopping:
		return TimelineSlotStarting
	case model.StatusFailed:
		return TimelineSlotFailed
	case model.StatusStopped:
		return TimelineSlotStopped
	default:
		return TimelineSlotEmpty
	}
}
