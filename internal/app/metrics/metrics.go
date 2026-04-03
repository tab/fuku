package metrics

import (
	"context"

	"fuku/internal/app/bus"
	"fuku/internal/config/sentry"
)

// Collector subscribes to bus events and emits metrics
type Collector interface {
	Run(ctx context.Context)
}

type collector struct {
	bus bus.Bus
}

// NewCollector creates a new metrics collector
func NewCollector(bus bus.Bus) Collector {
	return &collector{bus: bus}
}

// Run subscribes to the bus and emits metrics for each relevant event
func (c *collector) Run(ctx context.Context) {
	ch := c.bus.Subscribe(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			c.handle(ctx, msg)
		}
	}
}

func (c *collector) handle(ctx context.Context, msg bus.Message) {
	//nolint:exhaustive // only handling events relevant to metrics
	switch msg.Type {
	case bus.EventProfileResolved:
		c.handleProfileResolved(ctx, msg)
	case bus.EventTierReady:
		c.handleTierReady(ctx, msg)
	case bus.EventReadinessComplete:
		c.handleReadinessComplete(ctx, msg)
	case bus.EventServiceReady:
		c.handleServiceReady(ctx, msg)
	case bus.EventServiceFailed:
		c.handleServiceFailed(ctx)
	case bus.EventServiceRestarting:
		c.handleServiceRestarting(ctx)
	case bus.EventWatchTriggered:
		c.handleWatchTriggered(ctx)
	case bus.EventPreflightComplete:
		c.handlePreflightComplete(ctx, msg)
	case bus.EventServiceStopped:
		c.handleServiceStopped(ctx, msg)
	case bus.EventCommandStarted:
		c.handleCommandStarted(ctx, msg)
	case bus.EventPhaseChanged:
		c.handlePhaseChanged(ctx, msg)
	case bus.EventResourceSample:
		c.handleResourceSample(ctx, msg)
	case bus.EventAPIStarted:
		c.handleAPIStarted(ctx)
	}
}

func (c *collector) handleCommandStarted(ctx context.Context, msg bus.Message) {
	data, ok := msg.Data.(bus.CommandStarted)
	if !ok {
		return
	}

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag(sentry.TagCommand, data.Command)
		scope.SetTag(sentry.TagProfile, data.Profile)
	})

	sentry.NewMeter(ctx).Count(sentry.MetricAppRun, 1,
		sentry.WithAttributes(
			sentry.StringAttr(sentry.TagCommand, data.Command),
			sentry.BoolAttr(sentry.TagUI, data.UI),
		),
	)
}

func (c *collector) handleProfileResolved(ctx context.Context, msg bus.Message) {
	data, ok := msg.Data.(bus.ProfileResolved)
	if !ok {
		return
	}

	serviceCount := 0
	for _, tier := range data.Tiers {
		serviceCount += len(tier.Services)
	}

	meter := sentry.NewMeter(ctx)
	meter.Gauge(sentry.MetricServiceCount, float64(serviceCount))
	meter.Gauge(sentry.MetricTierCount, float64(len(data.Tiers)))
	meter.Distribution(sentry.MetricDiscoveryDuration, float64(data.Duration.Milliseconds()),
		sentry.WithUnit(sentry.UnitMillisecond),
		sentry.WithAttributes(sentry.IntAttr(sentry.TagServiceCount, serviceCount)),
	)
}

func (c *collector) handleReadinessComplete(ctx context.Context, msg bus.Message) {
	data, ok := msg.Data.(bus.ReadinessComplete)
	if !ok {
		return
	}

	sentry.NewMeter(ctx).Distribution(sentry.MetricReadinessDuration, float64(data.Duration.Milliseconds()),
		sentry.WithUnit(sentry.UnitMillisecond),
		sentry.WithAttributes(sentry.StringAttr(sentry.TagType, data.Type)),
	)
}

func (c *collector) handleTierReady(ctx context.Context, msg bus.Message) {
	data, ok := msg.Data.(bus.TierReady)
	if !ok {
		return
	}

	sentry.NewMeter(ctx).Distribution(sentry.MetricTierStartupDuration, float64(data.Duration.Milliseconds()),
		sentry.WithUnit(sentry.UnitMillisecond),
		sentry.WithAttributes(sentry.IntAttr(sentry.TagServiceCount, data.ServiceCount)),
	)
}

func (c *collector) handleServiceReady(ctx context.Context, msg bus.Message) {
	data, ok := msg.Data.(bus.ServiceReady)
	if !ok {
		return
	}

	sentry.NewMeter(ctx).Distribution(sentry.MetricServiceStartupDuration, float64(data.Duration.Milliseconds()),
		sentry.WithUnit(sentry.UnitMillisecond),
	)
}

func (c *collector) handleServiceFailed(ctx context.Context) {
	sentry.NewMeter(ctx).Count(sentry.MetricServiceFailed, 1)
}

func (c *collector) handleServiceRestarting(ctx context.Context) {
	sentry.NewMeter(ctx).Count(sentry.MetricServiceRestart, 1)
}

func (c *collector) handleWatchTriggered(ctx context.Context) {
	sentry.NewMeter(ctx).Count(sentry.MetricWatchRestart, 1)
}

func (c *collector) handlePreflightComplete(ctx context.Context, msg bus.Message) {
	data, ok := msg.Data.(bus.PreflightComplete)
	if !ok {
		return
	}

	meter := sentry.NewMeter(ctx)
	meter.Gauge(sentry.MetricPreflightKilled, float64(data.Killed))
	meter.Distribution(sentry.MetricPreflightDuration, float64(data.Duration.Milliseconds()),
		sentry.WithUnit(sentry.UnitMillisecond),
	)
}

func (c *collector) handleServiceStopped(ctx context.Context, msg bus.Message) {
	data, ok := msg.Data.(bus.ServiceStopped)
	if !ok {
		return
	}

	if data.Unexpected {
		sentry.NewMeter(ctx).Count(sentry.MetricUnexpectedExit, 1)
	}
}

func (c *collector) handleResourceSample(ctx context.Context, msg bus.Message) {
	data, ok := msg.Data.(bus.ResourceSample)
	if !ok {
		return
	}

	meter := sentry.NewMeter(ctx)
	meter.Distribution(sentry.MetricFukuCPU, data.CPU, sentry.WithUnit(sentry.UnitPercent))
	meter.Distribution(sentry.MetricFukuMemory, data.MEM, sentry.WithUnit(sentry.UnitMegabyte))
}

func (c *collector) handleAPIStarted(ctx context.Context) {
	sentry.NewMeter(ctx).Count(sentry.MetricAPIEnabled, 1)
}

func (c *collector) handlePhaseChanged(ctx context.Context, msg bus.Message) {
	data, ok := msg.Data.(bus.PhaseChanged)
	if !ok {
		return
	}

	if data.Duration <= 0 {
		return
	}

	//nolint:exhaustive // only emitting metrics for running and stopped phases
	switch data.Phase {
	case bus.PhaseRunning:
		sentry.NewMeter(ctx).Distribution(sentry.MetricStartupDuration, float64(data.Duration.Milliseconds()),
			sentry.WithUnit(sentry.UnitMillisecond),
			sentry.WithAttributes(sentry.IntAttr(sentry.TagServiceCount, data.ServiceCount)),
		)
	case bus.PhaseStopped:
		sentry.NewMeter(ctx).Distribution(sentry.MetricShutdownDuration, float64(data.Duration.Milliseconds()),
			sentry.WithUnit(sentry.UnitMillisecond),
			sentry.WithAttributes(sentry.IntAttr(sentry.TagServiceCount, data.ServiceCount)),
		)
	}
}
