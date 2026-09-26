package telemetry

import (
	"context"
	"fmt"
	"sync"

	"github.com/getsentry/sentry-go"

	"fuku/internal/contracts"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

// Tracer turns the run lifecycle it sees on the bus into one Sentry transaction with child spans
type Tracer struct {
	subscriber contracts.Subscriber
	loop       *contracts.Loop
	mu         sync.Mutex
	trace      *sentry.Span
	tiers      []model.Tier
	tierIndex  map[string]int
}

// NewTracer creates a new bus-driven tracer
func NewTracer(subscriber contracts.Subscriber) *Tracer {
	return &Tracer{
		subscriber: subscriber,
	}
}

func (t *Tracer) handle(ctx context.Context, msg contracts.Message) {
	t.mu.Lock()
	defer t.mu.Unlock()

	//nolint:exhaustive // only handling events relevant to tracing
	switch msg.Type {
	case contracts.EventCommandStarted:
		t.handleCommandStarted(ctx, msg)
	case contracts.EventProfileResolved:
		t.handleProfileResolved(msg)
	case contracts.EventPreflightComplete:
		t.handlePreflightComplete(msg)
	case contracts.EventTierReady:
		t.handleTierReady(msg)
	case contracts.EventWatchTriggered:
		t.createSpan(OpWatchRestart)
	case contracts.CommandStopService:
		t.createSpan(OpServiceStop)
	case contracts.CommandRestartService:
		t.createSpan(OpServiceRestart)
	case contracts.EventPhaseChanged:
		t.handlePhaseChanged(msg)
	}
}

func (t *Tracer) handleCommandStarted(ctx context.Context, msg contracts.Message) {
	data, ok := msg.Data.(contracts.CommandStarted)
	if !ok {
		return
	}

	if data.Command != contracts.CommandNameRun {
		return
	}

	t.trace = sentry.StartTransaction(ctx, fmt.Sprintf("%s %s", buildinfo.AppName, data.Command),
		sentry.WithTransactionSource(sentry.SourceTask),
		withStartTime(msg.Timestamp),
	)
}

func (t *Tracer) handleProfileResolved(msg contracts.Message) {
	data, ok := msg.Data.(contracts.ProfileResolved)
	if !ok || t.trace == nil {
		return
	}

	t.tiers = data.Tiers
	t.tierIndex = make(map[string]int, len(data.Tiers))

	for i, tier := range data.Tiers {
		t.tierIndex[tier.Name] = i
	}

	span := t.trace.StartChild(OpDiscovery,
		withStartTime(msg.Timestamp.Add(-data.Duration)),
	)
	span.Finish()
}

func (t *Tracer) handlePreflightComplete(msg contracts.Message) {
	data, ok := msg.Data.(contracts.PreflightComplete)
	if !ok || t.trace == nil {
		return
	}

	span := t.trace.StartChild(OpPreflight,
		withStartTime(msg.Timestamp.Add(-data.Duration)),
	)
	span.Finish()
}

func (t *Tracer) handleTierReady(msg contracts.Message) {
	data, ok := msg.Data.(contracts.TierReady)
	if !ok || t.trace == nil {
		return
	}

	index, total := t.tierPosition(data.Name)

	span := t.trace.StartChild(OpTierStartup,
		withStartTime(msg.Timestamp.Add(-data.Duration)),
		sentry.WithDescription(fmt.Sprintf("tier %d/%d (%d services)", index, total, data.ServiceCount)),
	)
	span.Finish()
}

func (t *Tracer) tierPosition(name string) (int, int) {
	if i, exists := t.tierIndex[name]; exists {
		return i + 1, len(t.tiers)
	}

	return 0, len(t.tiers)
}

func (t *Tracer) handlePhaseChanged(msg contracts.Message) {
	data, ok := msg.Data.(contracts.PhaseChanged)
	if !ok || t.trace == nil {
		return
	}

	if data.Phase != model.PhaseStopped {
		return
	}

	if data.Duration > 0 {
		span := t.trace.StartChild(OpShutdown,
			withStartTime(msg.Timestamp.Add(-data.Duration)),
		)
		span.Finish()
	}

	t.finish(sentry.SpanStatusOK)
}

func (t *Tracer) createSpan(op string) {
	if t.trace == nil {
		return
	}

	span := t.trace.StartChild(op)
	span.Finish()
}

func (t *Tracer) finish(status sentry.SpanStatus) {
	if t.trace == nil {
		return
	}

	t.trace.Status = status
	t.trace.Finish()
	t.trace = nil
}
