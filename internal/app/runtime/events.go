package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"fuku/internal/app/logs"
	"fuku/internal/config/logger"
)

// EventType represents the type of event
type EventType string

// Event types for runtime notifications
const (
	EventProfileResolved   EventType = "profile_resolved"
	EventPhaseChanged      EventType = "phase_changed"
	EventTierStarting      EventType = "tier_starting"
	EventTierReady         EventType = "tier_ready"
	EventServiceStarting   EventType = "service_starting"
	EventServiceReady      EventType = "service_ready"
	EventServiceFailed     EventType = "service_failed"
	EventServiceStopping   EventType = "service_stopping"
	EventServiceStopped    EventType = "service_stopped"
	EventServiceRestarting EventType = "service_restarting"
	EventSignalCaught      EventType = "signal_caught"
	EventWatchTriggered    EventType = "watch_triggered"
	EventWatchStarted      EventType = "watch_started"
	EventWatchStopped      EventType = "watch_stopped"
)

// Phase represents the application phase
type Phase string

// Phase values for application lifecycle
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

// ServiceStoppingData contains service stopping details
type ServiceStoppingData struct {
	Service string
	Tier    string
}

// ServiceStoppedData contains service stop details
type ServiceStoppedData struct {
	Service string
	Tier    string
}

// ServiceRestartingData contains service restart details
type ServiceRestartingData struct {
	Service string
	Tier    string
}

// SignalCaughtData contains signal details
type SignalCaughtData struct {
	Signal string
}

// WatchTriggeredData contains file watch trigger details
type WatchTriggeredData struct {
	Service      string
	ChangedFiles []string
}

// WatchStartedData contains watch started details
type WatchStartedData struct {
	Service string
}

// WatchStoppedData contains watch stopped details
type WatchStoppedData struct {
	Service string
}

// EventBus defines the interface for event publishing and subscription
type EventBus interface {
	Subscribe(ctx context.Context) <-chan Event
	Publish(event Event)
	SetBroadcaster(broadcaster logs.Broadcaster)
	Close()
}

// eventBus implements the EventBus interface
type eventBus struct {
	subscribers []chan Event
	mu          sync.RWMutex
	bufferSize  int
	closed      bool
	log         logger.Logger
	broadcaster logs.Broadcaster
}

// NewEventBus creates a new event bus with the specified buffer size
func NewEventBus(bufferSize int) EventBus {
	return NewEventBusWithLogger(bufferSize, nil)
}

// NewEventBusWithLogger creates a new event bus with logging support
func NewEventBusWithLogger(bufferSize int, log logger.Logger) EventBus {
	return &eventBus{
		subscribers: make([]chan Event, 0),
		bufferSize:  bufferSize,
		log:         log,
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

	if eb.log != nil {
		msg := fmt.Sprintf("EVENT %s %s", event.Type, formatEventData(event.Data))
		eb.log.Debug().Msg(msg)

		if eb.broadcaster != nil {
			eb.broadcaster.Broadcast("fuku", msg)
		}
	}

	for _, ch := range eb.subscribers {
		select {
		case ch <- event:
		default:
			if event.Critical {
				go func(c chan Event, e Event) {
					defer func() {
						recover()
					}()

					c <- e
				}(ch, event)
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

// SetBroadcaster sets the broadcaster for streaming internal logs
func (eb *eventBus) SetBroadcaster(broadcaster logs.Broadcaster) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.broadcaster = broadcaster
}

// unsubscribe removes a channel from subscribers and closes it
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

// formatEventData formats event data for debug logging
func formatEventData(data interface{}) string {
	switch d := data.(type) {
	case ProfileResolvedData:
		return fmt.Sprintf("{profile: %s}", d.Profile)
	case PhaseChangedData:
		return fmt.Sprintf("{phase: %s}", d.Phase)
	case TierStartingData:
		return fmt.Sprintf("{tier: %s, %d/%d}", d.Name, d.Index, d.Total)
	case TierReadyData:
		return fmt.Sprintf("{tier: %s}", d.Name)
	case ServiceStartingData:
		return fmt.Sprintf("{service: %s, tier: %s, pid: %d}", d.Service, d.Tier, d.PID)
	case ServiceReadyData:
		return fmt.Sprintf("{service: %s, tier: %s}", d.Service, d.Tier)
	case ServiceFailedData:
		return fmt.Sprintf("{service: %s, tier: %s, error: %v}", d.Service, d.Tier, d.Error)
	case ServiceStoppingData:
		return fmt.Sprintf("{service: %s, tier: %s}", d.Service, d.Tier)
	case ServiceStoppedData:
		return fmt.Sprintf("{service: %s, tier: %s}", d.Service, d.Tier)
	case ServiceRestartingData:
		return fmt.Sprintf("{service: %s, tier: %s}", d.Service, d.Tier)
	case SignalCaughtData:
		return fmt.Sprintf("{signal: %s}", d.Signal)
	case WatchTriggeredData:
		return fmt.Sprintf("{service: %s, files: %v}", d.Service, d.ChangedFiles)
	case WatchStartedData:
		return fmt.Sprintf("{service: %s}", d.Service)
	case WatchStoppedData:
		return fmt.Sprintf("{service: %s}", d.Service)
	default:
		return fmt.Sprintf("%+v", data)
	}
}

// NoOpEventBus is a no-operation event bus for when UI is disabled
type noOpEventBus struct{}

// NewNoOpEventBus creates a no-op event bus
func NewNoOpEventBus() EventBus {
	return &noOpEventBus{}
}

// Subscribe returns an immediately closed channel
func (neb *noOpEventBus) Subscribe(ctx context.Context) <-chan Event {
	ch := make(chan Event)
	close(ch)

	return ch
}

// Publish is a no-op
func (neb *noOpEventBus) Publish(event Event) {}

// SetBroadcaster is a no-op
func (neb *noOpEventBus) SetBroadcaster(broadcaster logs.Broadcaster) {}

// Close is a no-op
func (neb *noOpEventBus) Close() {}
