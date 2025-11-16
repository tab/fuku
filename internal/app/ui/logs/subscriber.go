package logs

import (
	"context"

	"fuku/internal/app/runtime"
	"fuku/internal/app/ui"
)

// Subscriber forwards log events from EventBus to LogView
type Subscriber struct {
	eventBus runtime.EventBus
	logView  ui.LogView
}

// NewSubscriber creates a new log subscriber
func NewSubscriber(eventBus runtime.EventBus, logView ui.LogView) *Subscriber {
	return &Subscriber{
		eventBus: eventBus,
		logView:  logView,
	}
}

// Start begins listening for log events
func (s *Subscriber) Start(ctx context.Context) {
	eventChan := s.eventBus.Subscribe(ctx)

	go s.processEvents(eventChan)
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

		entry := ui.LogEntry{
			Timestamp: event.Timestamp,
			Service:   data.Service,
			Tier:      data.Tier,
			Stream:    data.Stream,
			Message:   data.Message,
		}

		s.logView.HandleLog(entry)
	}
}
