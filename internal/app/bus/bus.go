package bus

import (
	"context"
	"sync"
	"time"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// MessageType represents the type of message
type MessageType string

// Event types
const (
	EventCommandStarted    MessageType = "command_started"
	EventPhaseChanged      MessageType = "phase_changed"
	EventProfileResolved   MessageType = "profile_resolved"
	EventPreflightStarted  MessageType = "preflight_started"
	EventPreflightKill     MessageType = "preflight_kill"
	EventPreflightComplete MessageType = "preflight_complete"
	EventTierStarting      MessageType = "tier_starting"
	EventTierReady         MessageType = "tier_ready"
	EventServiceStarting   MessageType = "service_starting"
	EventReadinessComplete MessageType = "readiness_complete"
	EventServiceReady      MessageType = "service_ready"
	EventServiceFailed     MessageType = "service_failed"
	EventServiceStopping   MessageType = "service_stopping"
	EventServiceStopped    MessageType = "service_stopped"
	EventServiceRestarting MessageType = "service_restarting"
	EventSignal            MessageType = "signal"
	EventWatchTriggered    MessageType = "watch_triggered"
	EventWatchStarted      MessageType = "watch_started"
	EventWatchStopped      MessageType = "watch_stopped"
	EventResourceSample    MessageType = "resource_sample"
	EventAPIStarted        MessageType = "api_started"
	EventAPIStopped        MessageType = "api_stopped"
	EventAPIRequest        MessageType = "api_request"
)

// Command types
const (
	CommandStartService   MessageType = "cmd_start_service"
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
	Data      any
	Critical  bool
}

// CommandStarted indicates a CLI command has begun execution
type CommandStarted struct {
	Command string
	Profile string
	UI      bool
}

// ProfileResolved contains the resolved profile with its tier structure
type ProfileResolved struct {
	Profile  string
	Tiers    []Tier
	Duration time.Duration
}

// PhaseChanged indicates an application phase transition
type PhaseChanged struct {
	Phase        Phase
	Duration     time.Duration
	ServiceCount int
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
	Killed   int
	Duration time.Duration
}

// Service identifies a service in the system
type Service struct {
	ID   string
	Name string
}

// Tier represents a group of services that start together
type Tier struct {
	ID       string
	Name     string
	Services []Service
}

// TierStarting indicates a tier is beginning its startup sequence
type TierStarting struct {
	Name string
}

// TierReady indicates a tier has completed startup
type TierReady struct {
	Name         string
	Duration     time.Duration
	ServiceCount int
}

// ServiceEvent is the base struct for service-related events
type ServiceEvent struct {
	Service Service
	Tier    string
}

// ServiceName returns the service name
func (e ServiceEvent) ServiceName() string {
	return e.Service.Name
}

// ServiceID returns the service UUID
func (e ServiceEvent) ServiceID() string {
	return e.Service.ID
}

// ServiceStarting indicates a service is starting with attempt and process info
type ServiceStarting struct {
	ServiceEvent
	Attempt int
	PID     int
}

// ReadinessComplete indicates a readiness check has finished successfully
type ReadinessComplete struct {
	Service  Service
	Type     string
	Duration time.Duration
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
	Unexpected bool
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
	Service      Service
	ChangedFiles []string
}

// ResourceSample contains fuku process CPU and memory readings
type ResourceSample struct {
	CPU float64
	MEM float64
}

// APIStarted indicates the API server has started listening
type APIStarted struct {
	Listen string
}

// APIStopped indicates the API server has shut down
type APIStopped struct{}

// APIRequest contains metrics for a completed API request
type APIRequest struct {
	Method   string
	Path     string
	Status   int
	Duration time.Duration
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
	formatter   *Formatter
	log         logger.Logger
}

// NewBus creates a new Bus
func NewBus(cfg *config.Config, formatter *Formatter, log logger.Logger) Bus {
	return &bus{
		cfg:         cfg,
		subscribers: make([]chan Message, 0),
		formatter:   formatter,
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

	if b.formatter != nil && b.log != nil {
		text := b.formatter.Format(msg.Type, msg.Data)
		b.log.Debug().Msg(text)
	}

	for _, ch := range b.subscribers {
		select {
		case ch <- msg:
		default:
			if msg.Critical {
				go func(c chan Message, m Message) {
					defer func() {
						//nolint:errcheck // recover return value is intentionally unused
						recover()
					}()

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
