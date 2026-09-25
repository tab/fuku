package bus

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
		sub.stop()
		b.report(sub, sub.close())
	}
}
