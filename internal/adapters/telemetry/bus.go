package telemetry

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"

	"fuku/internal/contracts"
)

// Subscribe registers the subscription and emits a metric for each relevant event until ctx is cancelled
func (c *Collector) Subscribe(ctx context.Context) error {
	sub, err := c.subscriber.Subscribe(ctx, contracts.SubscribeOptions{Name: "metrics"})
	if err != nil {
		return fmt.Errorf("failed to subscribe the metrics collector: %w", err)
	}

	c.loop = contracts.Run(ctx, sub, func(msg contracts.Message) {
		c.handle(ctx, msg)
	})

	return nil
}

// Drain returns once the queue is empty and no handler is in flight
func (c *Collector) Drain(ctx context.Context) error {
	return c.loop.Drain(ctx)
}

// Subscribe registers the subscription and records spans until ctx is cancelled
func (t *Tracer) Subscribe(ctx context.Context) error {
	sub, err := t.subscriber.Subscribe(ctx, contracts.SubscribeOptions{Name: "tracer"})
	if err != nil {
		return fmt.Errorf("failed to subscribe the tracer: %w", err)
	}

	t.loop = contracts.Run(ctx, sub, func(msg contracts.Message) {
		t.handle(ctx, msg)
	})

	return nil
}

// Drain returns once the queue is empty and no handler is in flight, then cancels a transaction the run left open
func (t *Tracer) Drain(ctx context.Context) error {
	err := t.loop.Drain(ctx)

	t.mu.Lock()
	t.finish(sentry.SpanStatusCanceled)
	t.mu.Unlock()

	return err
}
