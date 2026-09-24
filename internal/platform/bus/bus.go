package bus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"fuku/internal/contracts"
)

// FailureReporter receives a rejected critical publish
type FailureReporter interface {
	Fail(err error)
}

// Logger is the logging surface the bus writes through
type Logger interface {
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Bus is the message transport with per-subscription FIFO queues
type Bus struct {
	options     Options
	reporter    FailureReporter
	mu          sync.RWMutex
	subscribers []*subscriber
	closed      bool
	publishMu   sync.Mutex
	log         Logger
}

// NewBus creates a new Bus
func NewBus(options Options, reporter FailureReporter, log Logger) *Bus {
	return &Bus{
		options:  options,
		reporter: reporter,
		log:      log,
	}
}

// Subscribe registers a named subscription and removes it when ctx is cancelled
func (b *Bus) Subscribe(ctx context.Context, opts contracts.SubscribeOptions) (contracts.Subscription, error) {
	if opts.Name == "" {
		return nil, ErrUnnamedSubscription
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, contracts.ErrBusClosed
	}

	sub := newSubscriber(opts, b.options.QueueDepth)
	b.subscribers = append(b.subscribers, sub)

	go func() {
		<-ctx.Done()
		b.unsubscribe(sub)
	}()

	return sub, nil
}

// Publish delivers a message to every matching subscription or rejects a critical one it cannot deliver
func (b *Bus) Publish(msg contracts.Message) error {
	blocked, err := b.admit(msg)
	if blocked == "" {
		return err
	}

	b.log.Error("Bus rejected a critical message", "error", err, "type", string(msg.Type), "subscription", blocked)
	b.reporter.Fail(err)

	return err
}

// admit delivers the message under the publish lock and names the subscription that blocked it
func (b *Bus) admit(msg contracts.Message) (string, error) {
	b.publishMu.Lock()
	defer b.publishMu.Unlock()

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed && msg.Type.Critical() {
		return "", contracts.ErrBusClosed
	}

	if b.closed {
		return "", nil
	}

	if blocked := b.reserve(msg.Type); blocked != "" {
		return blocked, fmt.Errorf("%w: required subscription '%s' cannot accept %s", contracts.ErrBusOverloaded, blocked, msg.Type)
	}

	msg.Timestamp = time.Now()

	for _, sub := range b.subscribers {
		if sub.matches(msg.Type) {
			sub.send(msg)
		}
	}

	return "", nil
}

// reserve names the first required subscription that cannot accept a critical type
func (b *Bus) reserve(msgType contracts.MessageType) string {
	if !msgType.Critical() {
		return ""
	}

	// sync: only the publisher fills a queue, so a free slot stays free until the send
	for _, sub := range b.subscribers {
		if sub.required && sub.matches(msgType) && sub.full() {
			return sub.name
		}
	}

	return ""
}
