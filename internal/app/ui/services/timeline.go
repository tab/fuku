package services

import "time"

// TimelineSlot represents the state of a single timeline sample
type TimelineSlot uint8

// Timeline slot constants
const (
	SlotEmpty    TimelineSlot = iota // No observation recorded
	SlotRunning                      // Service is running
	SlotStarting                     // Service is starting, restarting, or stopping
	SlotFailed                       // Service has failed
	SlotStopped                      // Service is stopped
)

// Timeline is a fixed-capacity ring buffer that records per-second service state samples
type Timeline struct {
	slots    []TimelineSlot
	capacity int
	index    int
	count    int
}

// NewTimeline creates a new Timeline with the given capacity
func NewTimeline(capacity int) *Timeline {
	if capacity < 1 {
		capacity = 1
	}

	return &Timeline{
		slots:    make([]TimelineSlot, capacity),
		capacity: capacity,
	}
}

// Append adds a sample to the timeline, dropping the oldest when full
func (t *Timeline) Append(slot TimelineSlot) {
	t.slots[t.index] = slot
	t.index = (t.index + 1) % t.capacity

	if t.count < t.capacity {
		t.count++
	}
}

// Count returns the number of observed samples
func (t *Timeline) Count() int {
	return t.count
}

// Slots returns the current window in chronological order, padded with SlotEmpty on the right for unobserved positions
func (t *Timeline) Slots() []TimelineSlot {
	result := make([]TimelineSlot, t.capacity)

	if t.count == 0 {
		return result
	}

	start := 0
	if t.count == t.capacity {
		start = t.index
	}

	for i := range t.count {
		result[i] = t.slots[(start+i)%t.capacity]
	}

	return result
}

// Backfill appends n samples of the given slot, capped by remaining capacity
func (t *Timeline) Backfill(slot TimelineSlot, n int) {
	for range min(n, t.capacity) {
		t.Append(slot)
	}
}

// backfillStartupHistory seeds missing amber slots for a service that became ready
func backfillStartupHistory(service *ServiceState, lifecycleSeq uint64, startedAt time.Time, readyAt time.Time) {
	if service.Timeline == nil || service.BackfilledSeq >= lifecycleSeq {
		return
	}

	if startedAt.IsZero() || !readyAt.After(startedAt) {
		return
	}

	service.BackfilledSeq = lifecycleSeq

	desired := max(1, int(readyAt.Sub(startedAt).Seconds()))
	missing := max(0, desired-service.StartupSampled)

	service.Timeline.Backfill(SlotStarting, missing)
	service.StartupSampled = max(service.StartupSampled, desired)
}

// sampleTimelines appends the current status of each service to its timeline
func (m *Model) sampleTimelines() {
	for _, service := range m.state.services {
		if service.Timeline == nil {
			continue
		}

		if service.StartTime.IsZero() && service.Timeline.Count() == 0 &&
			service.Status != StatusFailed &&
			service.Status != StatusStopped &&
			service.Status != StatusRestarting {
			continue
		}

		slot := StatusToSlot(service.Status)
		service.Timeline.Append(slot)

		if slot == SlotStarting {
			service.StartupSampled++
		}
	}
}

// StatusToSlot maps a service Status to the corresponding TimelineSlot
func StatusToSlot(status Status) TimelineSlot {
	switch status {
	case StatusRunning:
		return SlotRunning
	case StatusStarting, StatusRestarting, StatusStopping:
		return SlotStarting
	case StatusFailed:
		return SlotFailed
	case StatusStopped:
		return SlotStopped
	default:
		return SlotEmpty
	}
}
