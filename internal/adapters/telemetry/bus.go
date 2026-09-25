package telemetry

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"

	"fuku/internal/contracts"
)

// metricsTypes lists the events the collector turns into a measurement
var metricsTypes = []contracts.MessageType{
	contracts.EventProfileResolved,
	contracts.EventTierReady,
	contracts.EventReadinessComplete,
	contracts.EventServiceReady,
	contracts.EventServiceFailed,
	contracts.EventServiceRestarting,
	contracts.EventWatchTriggered,
	contracts.EventPreflightComplete,
	contracts.EventServiceStopped,
	contracts.EventCommandStarted,
	contracts.EventPhaseChanged,
	contracts.EventResourceSampled,
	contracts.EventAPIStarted,
	contracts.EventAPIStopped,
	contracts.EventAPIRequested,
}

// Subscribe registers the subscription and emits a metric for each relevant event until ctx is cancelled
func (c *Collector) Subscribe(ctx context.Context) error {
	sub, err := c.subscriber.Subscribe(ctx, contracts.SubscribeOptions{Name: "metrics", Types: metricsTypes})
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

// tracerTypes lists the events the tracer turns into a span
var tracerTypes = []contracts.MessageType{
	contracts.EventCommandStarted,
	contracts.EventProfileResolved,
	contracts.EventPreflightComplete,
	contracts.EventTierReady,
	contracts.EventWatchTriggered,
	contracts.CommandStopService,
	contracts.CommandRestartService,
	contracts.EventPhaseChanged,
}

// Subscribe registers the subscription and records spans until ctx is cancelled
func (t *Tracer) Subscribe(ctx context.Context) error {
	sub, err := t.subscriber.Subscribe(ctx, contracts.SubscribeOptions{Name: "tracer", Types: tracerTypes})
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
