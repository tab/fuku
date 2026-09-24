package logs

import (
	"fmt"
	"sync"

	"fuku/internal/model"
)

// Logger is the logging surface the log pipeline writes through
type Logger interface {
	Warn(msg string, args ...any)
}

// Subscription is one client's view of the stream: the matching history first, then live lines until it ends
type Subscription struct {
	services map[string]bool
	lines    chan model.LogLine
	dropped  int
}

// Lines returns the queue of the subscription (closed once the subscription ends)
func (s *Subscription) Lines() <-chan model.LogLine {
	return s.lines
}

// wants reports whether the subscription follows a service (no filter follows every service)
func (s *Subscription) wants(service string) bool {
	return len(s.services) == 0 || s.services[service]
}

// offer queues a line without blocking and counts it as dropped when the queue is full
func (s *Subscription) offer(line model.LogLine) {
	select {
	case s.lines <- line:
	default:
		s.dropped++
	}
}

// Hub keeps the recent history and fans live lines out to bounded per-subscription queues
type Hub struct {
	mu      sync.Mutex
	history *history
	queue   int
	subs    map[*Subscription]struct{}
	log     Logger
}

// NewHub creates the hub (a queue holds the whole history plus the buffer, so a replay never drops)
func NewHub(options Options, log Logger) *Hub {
	return &Hub{
		history: newHistory(options.History),
		queue:   options.Buffer + options.History,
		subs:    make(map[*Subscription]struct{}),
		log:     log,
	}
}

// Broadcast records a line and forwards it to every subscription that follows its service
func (h *Hub) Broadcast(service, message string) {
	line := model.LogLine{Service: service, Message: message}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.history.push(line)

	for sub := range h.subs {
		if sub.wants(service) {
			sub.offer(line)
		}
	}
}

// Subscribe replays the matching history into a new subscription, then feeds it live lines unless it asked not to
func (h *Hub) Subscribe(services []string, replay model.ReplayOptions) *Subscription {
	sub := &Subscription{
		services: make(map[string]bool, len(services)),
		lines:    make(chan model.LogLine, h.queue),
	}

	for _, service := range services {
		sub.services[service] = true
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, line := range h.history.replay(sub.wants, replay.Tail) {
		sub.offer(line)
	}

	if replay.NoFollow {
		close(sub.lines)

		return sub
	}

	h.subs[sub] = struct{}{}

	return sub
}

// Unsubscribe ends a live subscription, closes its queue and reports the lines it dropped
func (h *Hub) Unsubscribe(sub *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.subs[sub]; !ok {
		return
	}

	delete(h.subs, sub)
	close(sub.lines)

	if sub.dropped > 0 {
		h.log.Warn(fmt.Sprintf("Dropped %d log messages (buffer full)", sub.dropped))
	}
}
