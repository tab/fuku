package relay

import (
	"context"

	"fuku/internal/app/bus"
	"fuku/internal/config"
)

// Bridge forwards bus events to the relay broadcaster
type Bridge struct {
	b           bus.Bus
	broadcaster Broadcaster
	formatter   *bus.Formatter
	cancel      context.CancelFunc
}

// NewBridge creates a new bus-to-relay bridge
func NewBridge(b bus.Bus, broadcaster Broadcaster, formatter *bus.Formatter) *Bridge {
	return &Bridge{
		b:           b,
		broadcaster: broadcaster,
		formatter:   formatter,
	}
}

// Start subscribes to the bus synchronously and begins forwarding in a goroutine
func (br *Bridge) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	br.cancel = cancel

	ch := br.b.Subscribe(ctx)

	go br.forward(ctx, ch)
}

// Stop cancels the bridge context and unsubscribes from the bus
func (br *Bridge) Stop() {
	if br.cancel != nil {
		br.cancel()
	}
}

// forward reads bus events from the channel and broadcasts them
func (br *Bridge) forward(ctx context.Context, ch <-chan bus.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			text := br.formatter.Format(msg.Type, msg.Data)
			br.broadcaster.Broadcast(config.AppName, text)
		}
	}
}
