package logs

import "fuku/internal/model"

// history is a fixed-size ring of the most recent lines
type history struct {
	lines []model.LogLine
	head  int
	count int
}

// newHistory creates a ring keeping the given number of lines
func newHistory(capacity int) *history {
	return &history{lines: make([]model.LogLine, capacity)}
}

// push records a line, overwriting the oldest once the ring is full
func (h *history) push(line model.LogLine) {
	h.lines[h.head] = line
	h.head = (h.head + 1) % len(h.lines)

	if h.count < len(h.lines) {
		h.count++
	}
}

// replay returns the wanted lines oldest first, keeping only the newest tail when one is set
func (h *history) replay(wants func(service string) bool, tail *int) []model.LogLine {
	selected := make([]model.LogLine, 0, h.count)
	start := (h.head - h.count + len(h.lines)) % len(h.lines)

	for i := range h.count {
		line := h.lines[(start+i)%len(h.lines)]
		if wants(line.Service) {
			selected = append(selected, line)
		}
	}

	if tail != nil && len(selected) > *tail {
		selected = selected[len(selected)-*tail:]
	}

	return selected
}
