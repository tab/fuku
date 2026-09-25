package eventlog

import (
	"context"
	"fmt"

	"fuku/internal/contracts"
	"fuku/internal/platform/buildinfo"
)

// Broadcaster carries a formatted event to the log clients as a line of the fuku service
type Broadcaster interface {
	Broadcast(service, message string)
}

// Logger is the logging surface the recorder writes events through
type Logger interface {
	Debug(msg string, args ...any)
}

// unrecorded lists the read-model notifications, which would repeat every event and flood the log clients
var unrecorded = map[contracts.MessageType]bool{
	contracts.EventSnapshotChanged:         true,
	contracts.EventServiceResourcesSampled: true,
}

// Recorder writes every accepted bus event to the debug log and broadcasts it to the log clients
type Recorder struct {
	subscriber  contracts.Subscriber
	broadcaster Broadcaster
	formatter   *Formatter
	loop        *contracts.Loop
	log         Logger
}

// NewRecorder creates a new event recorder
func NewRecorder(subscriber contracts.Subscriber, broadcaster Broadcaster, formatter *Formatter, log Logger) *Recorder {
	return &Recorder{
		subscriber:  subscriber,
		broadcaster: broadcaster,
		formatter:   formatter,
		log:         log,
	}
}

// Subscribe registers the optional subscription and records events on its own goroutine until ctx is cancelled
func (r *Recorder) Subscribe(ctx context.Context) error {
	sub, err := r.subscriber.Subscribe(ctx, contracts.SubscribeOptions{Name: "eventlog"})
	if err != nil {
		return fmt.Errorf("failed to subscribe the event log: %w", err)
	}

	r.loop = contracts.Run(ctx, sub, r.record)

	return nil
}

// Drain records the queued events and returns once none is in flight (or once ctx expires)
func (r *Recorder) Drain(ctx context.Context) error {
	return r.loop.Drain(ctx)
}

// record logs one event and broadcasts it
func (r *Recorder) record(msg contracts.Message) {
	if unrecorded[msg.Type] {
		return
	}

	text := r.formatter.Format(msg.Type, msg.Data)

	r.log.Debug(text)
	r.broadcaster.Broadcast(buildinfo.AppName, text)
}
