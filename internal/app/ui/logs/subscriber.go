package logs

import (
	"context"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"fuku/internal/app/runtime"
	"fuku/internal/app/ui"
)

// LogMsg is a Bubble Tea message containing a log entry
type LogMsg ui.LogEntry

// Sender holds a function to send messages to Bubble Tea
type Sender struct {
	mu   sync.RWMutex
	send func(tea.Msg)
}

// NewSender creates a new Sender
func NewSender() *Sender {
	return &Sender{}
}

// Set sets the send function
func (s *Sender) Set(send func(tea.Msg)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.send = send
}

// Send sends a message if the send function is set
func (s *Sender) Send(msg tea.Msg) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.send != nil {
		s.send(msg)
	}
}

// Subscriber forwards log events from EventBus via Sender
type Subscriber struct {
	eventBus runtime.EventBus
	sender   *Sender
}

// NewSubscriber creates a new log subscriber
func NewSubscriber(eventBus runtime.EventBus, sender *Sender) *Subscriber {
	return &Subscriber{
		eventBus: eventBus,
		sender:   sender,
	}
}

// Start begins listening for log events
func (s *Subscriber) Start(ctx context.Context) {
	eventChan := s.eventBus.Subscribe(ctx)

	go s.processEvents(eventChan)
}

// StartCmd returns a Bubble Tea command that starts the subscriber
func (s *Subscriber) StartCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		s.Start(ctx)
		return nil
	}
}

func (s *Subscriber) processEvents(eventChan <-chan runtime.Event) {
	for event := range eventChan {
		if event.Type != runtime.EventLogLine {
			continue
		}

		data, ok := event.Data.(runtime.LogLineData)
		if !ok {
			continue
		}

		entry := LogMsg{
			Timestamp: event.Timestamp,
			Service:   data.Service,
			Tier:      data.Tier,
			Stream:    data.Stream,
			Message:   data.Message,
		}

		s.sender.Send(entry)
	}
}
