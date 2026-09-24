package bus

import (
	"fmt"

	"fuku/internal/contracts"
)

// Close ends every subscription and rejects further critical publishes
func (b *Bus) Close() {
	b.mu.Lock()

	if b.closed {
		b.mu.Unlock()

		return
	}

	b.closed = true
	ended := b.subscribers
	b.subscribers = nil

	b.mu.Unlock()

	for _, sub := range ended {
		b.report(sub, sub.close())
	}
}

// unsubscribe removes a subscription whose context ended
func (b *Bus) unsubscribe(sub *subscriber) {
	b.mu.Lock()

	index := -1

	for i, s := range b.subscribers {
		if s == sub {
			index = i

			break
		}
	}

	if index >= 0 {
		b.subscribers = append(b.subscribers[:index], b.subscribers[index+1:]...)
	}

	b.mu.Unlock()

	if index >= 0 {
		b.report(sub, sub.close())
	}
}

// report logs the drop counters of an ended subscription
func (b *Bus) report(sub *subscriber, drops map[contracts.MessageType]uint64) {
	for msgType, count := range drops {
		b.log.Warn(fmt.Sprintf("Subscription '%s' dropped %d %s messages", sub.name, count, msgType), "subscription", sub.name, "type", string(msgType), "dropped", count)
	}
}
