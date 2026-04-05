package tracer

import (
	"context"
	"fmt"
	"time"

	"fuku/internal/app/bus"
	"fuku/internal/config"
	"fuku/internal/config/sentry"
)

// Tracer subscribes to bus events and creates Sentry trace spans
type Tracer interface {
	Run(ctx context.Context, ch <-chan bus.Message)
}

type tracer struct {
	trace     *sentry.Span
	tiers     []bus.Tier
	tierIndex map[string]int
}

// NewTracer creates a new bus-driven tracer
func NewTracer() Tracer {
	return &tracer{}
}

// Run reads bus events from the provided channel and manages trace spans
func (t *tracer) Run(ctx context.Context, ch <-chan bus.Message) {
	for {
		select {
		case <-ctx.Done():
			t.finish(sentry.SpanStatusCanceled)

			return
		case msg, ok := <-ch:
			if !ok {
				t.finish(sentry.SpanStatusCanceled)

				return
			}

			t.handle(ctx, msg)
		}
	}
}

func (t *tracer) handle(ctx context.Context, msg bus.Message) {
	//nolint:exhaustive // only handling events relevant to tracing
	switch msg.Type {
	case bus.EventCommandStarted:
		t.handleCommandStarted(ctx, msg)
	case bus.EventProfileResolved:
		t.handleProfileResolved(msg)
	case bus.EventPreflightComplete:
		t.handlePreflightComplete(msg)
	case bus.EventTierReady:
		t.handleTierReady(msg)
	case bus.EventWatchTriggered:
		t.handleWatchTriggered()
	case bus.CommandStopService:
		t.createSpan(sentry.OpServiceStop)
	case bus.CommandRestartService:
		t.createSpan(sentry.OpServiceRestart)
	case bus.EventPhaseChanged:
		t.handlePhaseChanged(msg)
	}
}

func (t *tracer) handleCommandStarted(ctx context.Context, msg bus.Message) {
	data, ok := msg.Data.(bus.CommandStarted)
	if !ok {
		return
	}

	if data.Command != "run" {
		return
	}

	t.trace = sentry.StartTransaction(ctx, fmt.Sprintf("%s %s", config.AppName, data.Command),
		sentry.WithTransactionSource(sentry.SourceTask),
		withStartTime(msg.Timestamp),
	)
}

func (t *tracer) handleProfileResolved(msg bus.Message) {
	data, ok := msg.Data.(bus.ProfileResolved)
	if !ok || t.trace == nil {
		return
	}

	t.tiers = data.Tiers
	t.tierIndex = make(map[string]int, len(data.Tiers))

	for i, tier := range data.Tiers {
		t.tierIndex[tier.Name] = i
	}

	span := t.trace.StartChild(sentry.OpDiscovery,
		withStartTime(msg.Timestamp.Add(-data.Duration)),
	)
	span.Finish()
}

func (t *tracer) handlePreflightComplete(msg bus.Message) {
	data, ok := msg.Data.(bus.PreflightComplete)
	if !ok || t.trace == nil {
		return
	}

	span := t.trace.StartChild(sentry.OpPreflight,
		withStartTime(msg.Timestamp.Add(-data.Duration)),
	)
	span.Finish()
}

func (t *tracer) handleTierReady(msg bus.Message) {
	data, ok := msg.Data.(bus.TierReady)
	if !ok || t.trace == nil {
		return
	}

	index, total := t.tierPosition(data.Name)

	span := t.trace.StartChild(sentry.OpTierStartup,
		withStartTime(msg.Timestamp.Add(-data.Duration)),
		sentry.WithDescription(fmt.Sprintf("tier %d/%d (%d services)", index, total, data.ServiceCount)),
	)
	span.Finish()
}

func (t *tracer) tierPosition(name string) (int, int) {
	if i, exists := t.tierIndex[name]; exists {
		return i + 1, len(t.tiers)
	}

	return 0, len(t.tiers)
}

func (t *tracer) handleWatchTriggered() {
	t.createSpan(sentry.OpWatchRestart)
}

func (t *tracer) handlePhaseChanged(msg bus.Message) {
	data, ok := msg.Data.(bus.PhaseChanged)
	if !ok || t.trace == nil {
		return
	}

	if data.Phase != bus.PhaseStopped {
		return
	}

	if data.Duration > 0 {
		span := t.trace.StartChild(sentry.OpShutdown,
			withStartTime(msg.Timestamp.Add(-data.Duration)),
		)
		span.Finish()
	}

	t.finish(sentry.SpanStatusOK)
}

func (t *tracer) createSpan(op string) {
	if t.trace == nil {
		return
	}

	span := t.trace.StartChild(op)
	span.Finish()
}

func (t *tracer) finish(status sentry.SpanStatus) {
	if t.trace == nil {
		return
	}

	t.trace.Status = status
	t.trace.Finish()
	t.trace = nil
}

func withStartTime(ts time.Time) sentry.SpanOption {
	return func(s *sentry.Span) {
		s.StartTime = ts
	}
}
