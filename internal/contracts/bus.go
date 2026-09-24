package contracts

import "context"

// SubscribeOptions names a subscription and selects what it receives
type SubscribeOptions struct {
	Name     string
	Required bool          // a critical publish fails while this queue is full
	Types    []MessageType // nil means every type
}

// Subscription delivers messages in publication order
type Subscription interface {
	Messages() <-chan Message
}

// Publisher publishes a message to the bus (a critical publish fails with ErrBusOverloaded or ErrBusClosed)
type Publisher interface {
	Publish(msg Message) error
}

// Subscriber registers a named, filtered subscription on the bus
type Subscriber interface {
	Subscribe(ctx context.Context, opts SubscribeOptions) (Subscription, error)
}

// Loop handles a subscription on its own goroutine
type Loop struct {
	drain chan chan struct{}
	done  chan struct{}
}

// Run calls handle for every message until ctx is cancelled or the subscription closes
func Run(ctx context.Context, sub Subscription, handle func(Message)) *Loop {
	l := &Loop{
		drain: make(chan chan struct{}),
		done:  make(chan struct{}),
	}

	go l.run(ctx, sub.Messages(), handle)

	return l
}

// Done returns a channel that closes once the loop has exited and no handler is in flight
func (l *Loop) Done() <-chan struct{} {
	return l.done
}

// Drain waits until the queue is empty and no handler is in flight
func (l *Loop) Drain(ctx context.Context) error {
	idle := make(chan struct{})

	select {
	case l.drain <- idle:
	case <-l.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case <-idle:
		return nil
	case <-l.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run is the loop goroutine and the only receiver, so an empty channel means the queue is idle
func (l *Loop) run(ctx context.Context, messages <-chan Message, handle func(Message)) {
	defer close(l.done)

	var waiters []chan struct{}

	for {
		if len(waiters) > 0 && len(messages) == 0 {
			for _, waiter := range waiters {
				close(waiter)
			}

			waiters = nil
		}

		select {
		case <-ctx.Done():
			return
		case waiter := <-l.drain:
			waiters = append(waiters, waiter)
		case msg, ok := <-messages:
			if !ok {
				return
			}

			handle(msg)
		}
	}
}
