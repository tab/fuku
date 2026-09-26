package tui

import (
	"context"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"

	"fuku/internal/contracts"
)

// View receives the forwarded bus messages
type View interface {
	Send(msg tea.Msg)
}

// Bridge forwards bus messages to the program, holding them in the queue until the view attaches
type Bridge struct {
	subscriber contracts.Subscriber
	mu         sync.Mutex
	view       View
	attached   chan struct{}
	loop       *contracts.Loop
}

// NewBridge creates the bridge between the bus and the program
func NewBridge(subscriber contracts.Subscriber) *Bridge {
	return &Bridge{subscriber: subscriber, attached: make(chan struct{})}
}

// received lists the message types the view handles: snapshots, preflight progress, the signal and the update notice
var received = []contracts.MessageType{
	contracts.EventSnapshotChanged,
	contracts.EventPreflightStarted,
	contracts.EventPreflightKilled,
	contracts.EventPreflightComplete,
	contracts.EventSignalReceived,
	contracts.EventUpdateAvailable,
}

// Subscribe registers the optional subscription and forwards its messages on its own goroutine
func (b *Bridge) Subscribe(ctx context.Context) error {
	sub, err := b.subscriber.Subscribe(ctx, contracts.SubscribeOptions{Name: "tui", Types: received})
	if err != nil {
		return fmt.Errorf("failed to subscribe the services view: %w", err)
	}

	b.loop = contracts.Run(ctx, sub, func(msg contracts.Message) {
		b.forward(ctx, msg)
	})

	return nil
}

// Drain returns once the queued messages reached the view, or at once while no view is attached to receive them
func (b *Bridge) Drain(ctx context.Context) error {
	b.mu.Lock()
	attached := b.view != nil
	b.mu.Unlock()

	if !attached {
		return nil
	}

	return b.loop.Drain(ctx)
}

// attach hands the view to the bridge, releasing the messages queued while none was attached
func (b *Bridge) attach(view View) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.view = view
	close(b.attached)
}

// forward waits for a view and sends the message to it (the view drops a message once its program has exited)
func (b *Bridge) forward(ctx context.Context, msg contracts.Message) {
	select {
	case <-b.attached:
	case <-ctx.Done():
		return
	}

	b.mu.Lock()
	view := b.view
	b.mu.Unlock()

	view.Send(EventMsg(msg))
}
