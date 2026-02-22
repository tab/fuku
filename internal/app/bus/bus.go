package bus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"fuku/internal/app/logs"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// MessageType represents the type of message
type MessageType string

// Event types
const (
	EventPhaseChanged      MessageType = "phase_changed"
	EventProfileResolved   MessageType = "profile_resolved"
	EventPreflightStarted  MessageType = "preflight_started"
	EventPreflightKill     MessageType = "preflight_kill"
	EventPreflightComplete MessageType = "preflight_complete"
	EventTierStarting      MessageType = "tier_starting"
	EventTierReady         MessageType = "tier_ready"
	EventServiceStarting   MessageType = "service_starting"
	EventServiceReady      MessageType = "service_ready"
	EventServiceFailed     MessageType = "service_failed"
	EventServiceStopping   MessageType = "service_stopping"
	EventServiceStopped    MessageType = "service_stopped"
	EventServiceRestarting MessageType = "service_restarting"
	EventSignal            MessageType = "signal"
	EventWatchTriggered    MessageType = "watch_triggered"
	EventWatchStarted      MessageType = "watch_started"
	EventWatchStopped      MessageType = "watch_stopped"
)

// Command types
const (
	CommandStopService    MessageType = "cmd_stop_service"
	CommandRestartService MessageType = "cmd_restart_service"
	CommandStopAll        MessageType = "cmd_stop_all"
)

// Phase represents the application phase
type Phase string

const (
	PhaseStartup  Phase = "startup"
	PhaseRunning  Phase = "running"
	PhaseStopping Phase = "stopping"
	PhaseStopped  Phase = "stopped"
)

// Message represents a bus message (event or command)
type Message struct {
	Type      MessageType
	Timestamp time.Time
	Data      interface{}
	Critical  bool
}

// ProfileResolved contains the resolved profile with its tier structure
type ProfileResolved struct {
	Profile string
	Tiers   []Tier
}

// PhaseChanged indicates an application phase transition
type PhaseChanged struct {
	Phase Phase
}

// PreflightStarted indicates the preflight scan has begun
type PreflightStarted struct {
	Services []string
}

// PreflightKill indicates a process was killed during preflight
type PreflightKill struct {
	Service string
	Name    string
	PID     int
}

// PreflightComplete indicates the preflight scan has finished
type PreflightComplete struct {
	Killed int
}

// Tier represents a group of services that start together
type Tier struct {
	Name     string
	Services []string
}

// TierStarting indicates a tier is beginning its startup sequence
type TierStarting struct {
	Name  string
	Index int
	Total int
}

// Payload contains a simple name identifier for events
type Payload struct {
	Name string
}

// ServiceEvent is the base struct for service-related events
type ServiceEvent struct {
	Service string
	Tier    string
}

// ServiceStarting indicates a service is starting with attempt and process info
type ServiceStarting struct {
	ServiceEvent
	Attempt int
	PID     int
}

// ServiceReady indicates a service has completed startup and is ready
type ServiceReady struct {
	ServiceEvent
	Duration time.Duration
}

// ServiceFailed indicates a service failed to start or crashed
type ServiceFailed struct {
	ServiceEvent
	Error error
}

// ServiceStopping indicates a service is being stopped
type ServiceStopping struct {
	ServiceEvent
}

// ServiceStopped indicates a service has stopped
type ServiceStopped struct {
	ServiceEvent
}

// ServiceRestarting indicates a service is being restarted
type ServiceRestarting struct {
	ServiceEvent
}

// Signal contains information about a received OS signal
type Signal struct {
	Name string
}

// WatchTriggered indicates file changes detected for a watched service
type WatchTriggered struct {
	Service      string
	ChangedFiles []string
}

// Bus handles pub/sub messaging
type Bus interface {
	Subscribe(ctx context.Context) <-chan Message
	Publish(msg Message)
	Close()
}

// bus implements the Bus interface with pub/sub messaging
type bus struct {
	cfg         *config.Config
	subscribers []chan Message
	mu          sync.RWMutex
	closed      bool
	server      logs.Server
	log         logger.Logger
}

// New creates a new Bus
func New(cfg *config.Config, server logs.Server, log logger.Logger) Bus {
	return &bus{
		cfg:         cfg,
		subscribers: make([]chan Message, 0),
		server:      server,
		log:         log,
	}
}

// Subscribe creates a new subscription channel
func (b *bus) Subscribe(ctx context.Context) <-chan Message {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Message, b.cfg.Logs.Buffer)
	b.subscribers = append(b.subscribers, ch)

	go func() {
		<-ctx.Done()
		b.unsubscribe(ch)
	}()

	return ch
}

// Publish sends a message to all subscribers
func (b *bus) Publish(msg Message) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	msg.Timestamp = time.Now()

	text := fmt.Sprintf("%s %s", msg.Type, formatData(msg.Data))

	if b.log != nil {
		b.log.Debug().Msg(text)
	}

	if b.server != nil {
		b.server.Broadcast("fuku", text)
	}

	for _, ch := range b.subscribers {
		select {
		case ch <- msg:
		default:
			if msg.Critical {
				go func(c chan Message, m Message) {
					defer func() { recover() }()

					c <- m
				}(ch, msg)
			}
		}
	}
}

// Close closes all subscriber channels
func (b *bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.closed = true

	for _, ch := range b.subscribers {
		close(ch)
	}

	b.subscribers = nil
}

func (b *bus) unsubscribe(ch chan Message) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, sub := range b.subscribers {
		if sub == ch {
			b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)

			close(ch)

			break
		}
	}
}

func formatData(data interface{}) string {
	switch d := data.(type) {
	case ProfileResolved:
		return fmt.Sprintf("{profile: %s}", d.Profile)
	case PhaseChanged:
		return fmt.Sprintf("{phase: %s}", d.Phase)
	case PreflightStarted:
		return fmt.Sprintf("{services: %v}", d.Services)
	case PreflightKill:
		return fmt.Sprintf("{service: %s, pid: %d, name: %s}", d.Service, d.PID, d.Name)
	case PreflightComplete:
		return fmt.Sprintf("{killed: %d}", d.Killed)
	case TierStarting:
		return fmt.Sprintf("{tier: %s, %d/%d}", d.Name, d.Index, d.Total)
	case Payload:
		return fmt.Sprintf("{name: %s}", d.Name)
	case ServiceStarting:
		return fmt.Sprintf("{service: %s, tier: %s, pid: %d}", d.Service, d.Tier, d.PID)
	case ServiceReady:
		return fmt.Sprintf("{service: %s, tier: %s}", d.Service, d.Tier)
	case ServiceFailed:
		return fmt.Sprintf("{service: %s, tier: %s, error: %v}", d.Service, d.Tier, d.Error)
	case ServiceStopping:
		return fmt.Sprintf("{service: %s, tier: %s}", d.Service, d.Tier)
	case ServiceStopped:
		return fmt.Sprintf("{service: %s, tier: %s}", d.Service, d.Tier)
	case ServiceRestarting:
		return fmt.Sprintf("{service: %s, tier: %s}", d.Service, d.Tier)
	case Signal:
		return fmt.Sprintf("{signal: %s}", d.Name)
	case WatchTriggered:
		return fmt.Sprintf("{service: %s, files: %v}", d.Service, d.ChangedFiles)
	default:
		return fmt.Sprintf("%+v", data)
	}
}

// NoOp returns a no-op bus for when messaging is disabled
func NoOp() Bus {
	return &noOpBus{}
}

// noOpBus implements Bus interface with no-op methods for testing
type noOpBus struct{}

func (n *noOpBus) Subscribe(ctx context.Context) <-chan Message {
	ch := make(chan Message)

	go func() {
		<-ctx.Done()
		close(ch)
	}()

	return ch
}

func (n *noOpBus) Publish(msg Message) {}
func (n *noOpBus) Close()              {}
