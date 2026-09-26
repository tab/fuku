package bus

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	mu          sync.Mutex
	subscribers []*subscriber
	closed      bool
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
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, contracts.ErrBusClosed
	}

	sub := newSubscriber(opts, b.options.QueueDepth)
	sub.stop = context.AfterFunc(ctx, func() { b.unsubscribe(sub) })
	b.subscribers = append(b.subscribers, sub)

	return sub, nil
}

// unsubscribe removes a subscription whose context ended
func (b *Bus) unsubscribe(sub *subscriber) {
	b.mu.Lock()

	index := slices.Index(b.subscribers, sub)
	if index < 0 {
		b.mu.Unlock()

		return
	}

	b.subscribers = slices.Delete(b.subscribers, index, index+1)

	b.mu.Unlock()

	b.report(sub, sub.close())
}

// report logs the drop counters of an ended subscription
func (b *Bus) report(sub *subscriber, drops map[contracts.MessageType]uint64) {
	for msgType, count := range drops {
		b.log.Warn(fmt.Sprintf("Subscription '%s' dropped %d %s messages", sub.name, count, msgType), "subscription", sub.name, "type", string(msgType), "dropped", count)
	}
}

// Publish delivers a message to every matching subscription or rejects a critical one it cannot deliver
func (b *Bus) Publish(msg contracts.Message) error {
	err := b.admit(msg)
	if !errors.Is(err, contracts.ErrBusOverloaded) {
		return err
	}

	b.log.Error("Bus rejected a critical message", "error", err, "type", string(msg.Type))
	b.reporter.Fail(err)

	return err
}

// admit delivers the message under the lock or rejects it with the subscription that blocked it
func (b *Bus) admit(msg contracts.Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed && msg.Type.Critical() {
		return contracts.ErrBusClosed
	}

	if b.closed {
		return nil
	}

	if blocked := b.reserve(msg.Type); blocked != nil {
		return fmt.Errorf("%w: required subscription '%s' cannot accept %s", contracts.ErrBusOverloaded, blocked.name, msg.Type)
	}

	msg.Timestamp = time.Now()

	for _, sub := range b.subscribers {
		if sub.matches(msg.Type) {
			sub.send(msg)
		}
	}

	return nil
}

// reserve returns the first required subscription that cannot accept a critical type
func (b *Bus) reserve(msgType contracts.MessageType) *subscriber {
	if !msgType.Critical() {
		return nil
	}

	// sync: only the publisher fills a queue, so a free slot stays free until the send
	for _, sub := range b.subscribers {
		if sub.required && sub.matches(msgType) && sub.full() {
			return sub
		}
	}

	return nil
}
