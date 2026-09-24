package telemetry

import (
	"context"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// normalizePath replaces the service ID segment in API paths to bound metric cardinality
func normalizePath(path string) string {
	const prefix = "/api/v1/services/"

	_, rest, found := strings.Cut(path, prefix)
	if !found {
		return path
	}

	slash := strings.IndexByte(rest, '/')
	switch slash {
	case -1:
		return prefix + ":id"
	default:
		return prefix + ":id" + rest[slash:]
	}
}

// Collector emits a Sentry metric for every bus event that measures something
type Collector struct {
	subscriber contracts.Subscriber
	loop       *contracts.Loop
}

// NewCollector creates a new metrics collector
func NewCollector(subscriber contracts.Subscriber) *Collector {
	return &Collector{subscriber: subscriber}
}

func (c *Collector) handle(ctx context.Context, msg contracts.Message) {
	//nolint:exhaustive // only handling events relevant to metrics
	switch msg.Type {
	case contracts.EventProfileResolved:
		c.handleProfileResolved(ctx, msg)
	case contracts.EventTierReady:
		c.handleTierReady(ctx, msg)
	case contracts.EventReadinessComplete:
		c.handleReadinessComplete(ctx, msg)
	case contracts.EventServiceReady:
		c.handleServiceReady(ctx, msg)
	case contracts.EventServiceFailed:
		c.handleServiceFailed(ctx)
	case contracts.EventServiceRestarting:
		c.handleServiceRestarting(ctx)
	case contracts.EventWatchTriggered:
		c.handleWatchTriggered(ctx)
	case contracts.EventPreflightComplete:
		c.handlePreflightComplete(ctx, msg)
	case contracts.EventServiceStopped:
		c.handleServiceStopped(ctx, msg)
	case contracts.EventCommandStarted:
		c.handleCommandStarted(ctx, msg)
	case contracts.EventPhaseChanged:
		c.handlePhaseChanged(ctx, msg)
	case contracts.EventResourceSampled:
		c.handleResourceSample(ctx, msg)
	case contracts.EventAPIStarted:
		c.handleAPIStarted(ctx)
	case contracts.EventAPIStopped:
		c.handleAPIStopped(ctx)
	case contracts.EventAPIRequested:
		c.handleAPIRequest(ctx, msg)
	}
}

func (c *Collector) handleCommandStarted(ctx context.Context, msg contracts.Message) {
	data, ok := msg.Data.(contracts.CommandStarted)
	if !ok {
		return
	}

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag(TagCommand, data.Command)
		scope.SetTag(TagProfile, data.Profile)
	})

	sentry.NewMeter(ctx).Count(MetricAppRun, 1,
		sentry.WithAttributes(
			attribute.String(TagCommand, data.Command),
			attribute.Bool(TagUI, data.UI),
		),
	)
}

func (c *Collector) handleProfileResolved(ctx context.Context, msg contracts.Message) {
	data, ok := msg.Data.(contracts.ProfileResolved)
	if !ok {
		return
	}

	serviceCount := 0
	for _, tier := range data.Tiers {
		serviceCount += len(tier.Services)
	}

	meter := sentry.NewMeter(ctx)
	meter.Gauge(MetricServiceCount, float64(serviceCount))
	meter.Gauge(MetricTierCount, float64(len(data.Tiers)))
	meter.Distribution(MetricDiscoveryDuration, float64(data.Duration.Milliseconds()),
		sentry.WithUnit(sentry.UnitMillisecond),
		sentry.WithAttributes(attribute.Int(TagServiceCount, serviceCount)),
	)
}

func (c *Collector) handleReadinessComplete(ctx context.Context, msg contracts.Message) {
	data, ok := msg.Data.(contracts.ReadinessComplete)
	if !ok {
		return
	}

	sentry.NewMeter(ctx).Distribution(MetricReadinessDuration, float64(data.Duration.Milliseconds()),
		sentry.WithUnit(sentry.UnitMillisecond),
		sentry.WithAttributes(attribute.String(TagType, string(data.Type))),
	)
}

func (c *Collector) handleTierReady(ctx context.Context, msg contracts.Message) {
	data, ok := msg.Data.(contracts.TierReady)
	if !ok {
		return
	}

	sentry.NewMeter(ctx).Distribution(MetricTierStartupDuration, float64(data.Duration.Milliseconds()),
		sentry.WithUnit(sentry.UnitMillisecond),
		sentry.WithAttributes(attribute.Int(TagServiceCount, data.ServiceCount)),
	)
}

func (c *Collector) handleServiceReady(ctx context.Context, msg contracts.Message) {
	data, ok := msg.Data.(contracts.ServiceReady)
	if !ok {
		return
	}

	sentry.NewMeter(ctx).Distribution(MetricServiceStartupDuration, float64(data.Duration.Milliseconds()),
		sentry.WithUnit(sentry.UnitMillisecond),
	)
}

func (c *Collector) handleServiceFailed(ctx context.Context) {
	sentry.NewMeter(ctx).Count(MetricServiceFailed, 1)
}

func (c *Collector) handleServiceRestarting(ctx context.Context) {
	sentry.NewMeter(ctx).Count(MetricServiceRestart, 1)
}

func (c *Collector) handleWatchTriggered(ctx context.Context) {
	sentry.NewMeter(ctx).Count(MetricWatchRestart, 1)
}

func (c *Collector) handlePreflightComplete(ctx context.Context, msg contracts.Message) {
	data, ok := msg.Data.(contracts.PreflightComplete)
	if !ok {
		return
	}

	meter := sentry.NewMeter(ctx)
	meter.Gauge(MetricPreflightKilled, float64(data.Killed))
	meter.Distribution(MetricPreflightDuration, float64(data.Duration.Milliseconds()),
		sentry.WithUnit(sentry.UnitMillisecond),
	)
}

func (c *Collector) handleServiceStopped(ctx context.Context, msg contracts.Message) {
	data, ok := msg.Data.(contracts.ServiceStopped)
	if !ok {
		return
	}

	if data.Unexpected {
		sentry.NewMeter(ctx).Count(MetricUnexpectedExit, 1)
	}
}

func (c *Collector) handleResourceSample(ctx context.Context, msg contracts.Message) {
	data, ok := msg.Data.(contracts.ResourceSampled)
	if !ok {
		return
	}

	meter := sentry.NewMeter(ctx)
	meter.Distribution(MetricFukuCPU, data.CPU, sentry.WithUnit(sentry.UnitPercent))
	meter.Distribution(MetricFukuMemory, data.MEM, sentry.WithUnit(sentry.UnitMegabyte))
}

func (c *Collector) handleAPIStarted(ctx context.Context) {
	sentry.NewMeter(ctx).Gauge(MetricAPIEnabled, 1)
}

func (c *Collector) handleAPIStopped(ctx context.Context) {
	sentry.NewMeter(ctx).Gauge(MetricAPIEnabled, 0)
}

func (c *Collector) handleAPIRequest(ctx context.Context, msg contracts.Message) {
	data, ok := msg.Data.(contracts.APIRequested)
	if !ok {
		return
	}

	attrs := sentry.WithAttributes(
		attribute.String(TagMethod, data.Method),
		attribute.String(TagPath, normalizePath(data.Path)),
		attribute.Int(TagStatus, data.Status),
	)

	meter := sentry.NewMeter(ctx)
	meter.Count(MetricAPIRequests, 1, attrs)
	meter.Distribution(MetricAPIRequestDuration, float64(data.Duration.Milliseconds()),
		sentry.WithUnit(sentry.UnitMillisecond), attrs,
	)

	if data.Status == 401 {
		meter.Count(MetricAPIAuthFailures, 1)
	}
}

func (c *Collector) handlePhaseChanged(ctx context.Context, msg contracts.Message) {
	data, ok := msg.Data.(contracts.PhaseChanged)
	if !ok {
		return
	}

	if data.Duration <= 0 {
		return
	}

	//nolint:exhaustive // only emitting metrics for running and stopped phases
	switch data.Phase {
	case model.PhaseRunning:
		sentry.NewMeter(ctx).Distribution(MetricStartupDuration, float64(data.Duration.Milliseconds()),
			sentry.WithUnit(sentry.UnitMillisecond),
			sentry.WithAttributes(attribute.Int(TagServiceCount, data.ServiceCount)),
		)
	case model.PhaseStopped:
		sentry.NewMeter(ctx).Distribution(MetricShutdownDuration, float64(data.Duration.Milliseconds()),
			sentry.WithUnit(sentry.UnitMillisecond),
			sentry.WithAttributes(attribute.Int(TagServiceCount, data.ServiceCount)),
		)
	}
}
