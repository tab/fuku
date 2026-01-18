package runtime

import (
	"context"
	"sync"
	"time"
)

// EventType represents the type of event
type EventType string

const (
	EventProfileResolved EventType = "profile_resolved"
	EventPhaseChanged    EventType = "phase_changed"
	EventTierStarting    EventType = "tier_starting"
	EventTierReady       EventType = "tier_ready"
	EventTierFailed      EventType = "tier_failed"
	EventServiceStarting EventType = "service_starting"
	EventServiceReady    EventType = "service_ready"
	EventServiceFailed   EventType = "service_failed"
	EventServiceStopped  EventType = "service_stopped"
	EventRetryScheduled  EventType = "retry_scheduled"
	EventSignalCaught    EventType = "signal_caught"
)

// Phase represents the application phase
type Phase string

const (
	PhaseStartup  Phase = "startup"
	PhaseRunning  Phase = "running"
	PhaseStopping Phase = "stopping"
	PhaseStopped  Phase = "stopped"
)

// Event represents a runtime event
type Event struct {
	Type      EventType
	Timestamp time.Time
	Data      interface{}
	Critical  bool
}

// TierData represents a tier for event data
type TierData struct {
	Name     string
	Services []string
}

// ProfileResolvedData contains profile resolution details
type ProfileResolvedData struct {
	Profile string
	Tiers   []TierData
}

// PhaseChangedData contains phase transition details
type PhaseChangedData struct {
	Phase Phase
}

// TierStartingData contains tier startup details
type TierStartingData struct {
	Name  string
	Index int
	Total int
}

// TierReadyData contains tier ready details
type TierReadyData struct {
	Name string
}

// TierFailedData contains tier failure details
type TierFailedData struct {
	Name           string
	FailedServices []string
	TotalServices  int
}

// ServiceStartingData contains service startup details
type ServiceStartingData struct {
	Service string
	Tier    string
	Attempt int
	PID     int
}

// ServiceReadyData contains service ready details
type ServiceReadyData struct {
	Service  string
	Tier     string
	Duration time.Duration
}

// ServiceFailedData contains service failure details
type ServiceFailedData struct {
	Service string
	Tier    string
	Error   error
}

// ServiceStoppedData contains service stop details
type ServiceStoppedData struct {
	Service string
	Tier    string
}

// RetryScheduledData contains retry details
type RetryScheduledData struct {
	Service     string
	Attempt     int
	MaxAttempts int
}

// SignalCaughtData contains signal details
type SignalCaughtData struct {
	Signal string
}

// EventBus defines the interface for event publishing and subscription
type EventBus interface {
	Subscribe(ctx context.Context) <-chan Event
	Publish(event Event)
	Close()
}

type eventBus struct {
	subscribers []chan Event
	mu          sync.RWMutex
	bufferSize  int
	closed      bool
}

// NewEventBus creates a new event bus with the specified buffer size
func NewEventBus(bufferSize int) EventBus {
	return &eventBus{
		subscribers: make([]chan Event, 0),
		bufferSize:  bufferSize,
	}
}

// Subscribe creates a new subscription channel for events
func (eb *eventBus) Subscribe(ctx context.Context) <-chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan Event, eb.bufferSize)
	eb.subscribers = append(eb.subscribers, ch)

	go func() {
		<-ctx.Done()
		eb.unsubscribe(ch)
	}()

	return ch
}

// Publish sends an event to all subscribers
func (eb *eventBus) Publish(event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if eb.closed {
		return
	}

	event.Timestamp = time.Now()

	if event.Critical {
		for _, ch := range eb.subscribers {
			ch <- event
		}
	} else {
		for _, ch := range eb.subscribers {
			select {
			case ch <- event:
			default:
			}
		}
	}
}

// Close closes all subscriber channels
func (eb *eventBus) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		return
	}

	eb.closed = true

	for _, ch := range eb.subscribers {
		close(ch)
	}

	eb.subscribers = nil
}

func (eb *eventBus) unsubscribe(ch chan Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	for i, sub := range eb.subscribers {
		if sub == ch {
			eb.subscribers = append(eb.subscribers[:i], eb.subscribers[i+1:]...)

			close(ch)

			break
		}
	}
}

// NoOpEventBus is a no-operation event bus for when UI is disabled
type noOpEventBus struct{}

// NewNoOpEventBus creates a no-op event bus
func NewNoOpEventBus() EventBus {
	return &noOpEventBus{}
}

func (neb *noOpEventBus) Subscribe(ctx context.Context) <-chan Event {
	ch := make(chan Event)
	close(ch)

	return ch
}

func (neb *noOpEventBus) Publish(event Event) {}

func (neb *noOpEventBus) Close() {}
