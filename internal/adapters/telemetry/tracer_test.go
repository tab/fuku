package telemetry

import (
	"slices"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Tracer_Subscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	tracer := NewTracer(mockSubscriber)

	tests := []struct {
		name     string
		before   func()
		expected error
	}{
		{
			name: "registers the optional tracer subscription and handles its messages until it closes",
			before: func() {
				messages := make(queue, 2)
				messages <- contracts.Message{
					Type:      contracts.EventCommandStarted,
					Timestamp: time.Now(),
					Data:      contracts.CommandStarted{Command: "run", Profile: "default"},
				}

				messages <- contracts.Message{
					Type:      contracts.EventProfileResolved,
					Timestamp: time.Now(),
					Data: contracts.ProfileResolved{
						Profile:  "default",
						Tiers:    []model.Tier{{Name: "default", Services: []*model.Service{{ID: "test-id-api", Name: "api"}}}},
						Duration: 10 * time.Millisecond,
					},
				}

				close(messages)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "tracer", Types: tracerTypes}).Return(messages, nil)
			},
		},
		{
			name: "returns the subscribe error",
			before: func() {
				mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "tracer", Types: tracerTypes}).Return(nil, contracts.ErrBusClosed)
			},
			expected: contracts.ErrBusClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := tracer.Subscribe(t.Context())

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Tracer_Drain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	started := contracts.Message{
		Type:      contracts.EventCommandStarted,
		Timestamp: time.Now(),
		Data:      contracts.CommandStarted{Command: "run"},
	}
	stopped := contracts.Message{
		Type:      contracts.EventPhaseChanged,
		Timestamp: time.Now(),
		Data:      contracts.PhaseChanged{Phase: model.PhaseStopped},
	}

	tests := []struct {
		name     string
		before   func(t *testing.T) (*Tracer, *sentry.Span)
		expected sentry.SpanStatus
	}{
		{
			name: "a transaction the run left open is finished as cancelled",
			before: func(t *testing.T) (*Tracer, *sentry.Span) {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "tracer", Types: tracerTypes}).Return(make(queue), nil)
				require.NoError(t, tracer.Subscribe(t.Context()))

				return tracer, tracer.trace
			},
			expected: sentry.SpanStatusCanceled,
		},
		{
			name: "a transaction the run stopped keeps its status",
			before: func(t *testing.T) (*Tracer, *sentry.Span) {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)
				trace := tracer.trace
				tracer.handle(t.Context(), stopped)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "tracer", Types: tracerTypes}).Return(make(queue), nil)
				require.NoError(t, tracer.Subscribe(t.Context()))

				return tracer, trace
			},
			expected: sentry.SpanStatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer, trace := tt.before(t)

			err := tracer.Drain(t.Context())

			require.NoError(t, err)
			assert.Nil(t, tracer.trace)
			assert.Equal(t, tt.expected, trace.Status)
		})
	}
}

func Test_Tracer_handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	api := model.Service{ID: "test-id-api", Name: "api"}
	started := contracts.Message{
		Type:      contracts.EventCommandStarted,
		Timestamp: time.Now(),
		Data:      contracts.CommandStarted{Command: "run"},
	}
	resolved := contracts.Message{
		Type:      contracts.EventProfileResolved,
		Timestamp: time.Now(),
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers: []model.Tier{
				{Name: "foundation", Services: []*model.Service{{ID: "test-id-db", Name: "db"}, {ID: "test-id-cache", Name: "cache"}}},
				{Name: "app", Services: []*model.Service{&api}},
			},
			Duration: 50 * time.Millisecond,
		},
	}

	tests := []struct {
		name   string
		before func() *Tracer
		msg    contracts.Message
		traced bool
		tiers  int
	}{
		{
			name:   "command started opens the transaction",
			before: func() *Tracer { return NewTracer(mockSubscriber) },
			msg:    started,
			traced: true,
		},
		{
			name:   "command started with invalid data opens nothing",
			before: func() *Tracer { return NewTracer(mockSubscriber) },
			msg: contracts.Message{
				Type: contracts.EventCommandStarted,
				Data: "invalid",
			},
		},
		{
			name:   "a command other than run opens nothing",
			before: func() *Tracer { return NewTracer(mockSubscriber) },
			msg: contracts.Message{
				Type:      contracts.EventCommandStarted,
				Timestamp: time.Now(),
				Data:      contracts.CommandStarted{Command: "help"},
			},
		},
		{
			name: "profile resolved records the tiers",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg:    resolved,
			traced: true,
			tiers:  2,
		},
		{
			name: "profile resolved with invalid data keeps the transaction",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg: contracts.Message{
				Type: contracts.EventProfileResolved,
				Data: "invalid",
			},
			traced: true,
		},
		{
			name:   "profile resolved without a transaction records nothing",
			before: func() *Tracer { return NewTracer(mockSubscriber) },
			msg:    resolved,
		},
		{
			name: "preflight complete adds a span",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg: contracts.Message{
				Type:      contracts.EventPreflightComplete,
				Timestamp: time.Now(),
				Data:      contracts.PreflightComplete{Killed: 1, Duration: 20 * time.Millisecond},
			},
			traced: true,
		},
		{
			name: "preflight complete with invalid data keeps the transaction",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg: contracts.Message{
				Type: contracts.EventPreflightComplete,
				Data: "invalid",
			},
			traced: true,
		},
		{
			name:   "preflight complete without a transaction records nothing",
			before: func() *Tracer { return NewTracer(mockSubscriber) },
			msg: contracts.Message{
				Type:      contracts.EventPreflightComplete,
				Timestamp: time.Now(),
				Data:      contracts.PreflightComplete{Duration: 20 * time.Millisecond},
			},
		},
		{
			name: "tier ready adds a span",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)
				tracer.handle(t.Context(), resolved)

				return tracer
			},
			msg: contracts.Message{
				Type:      contracts.EventTierReady,
				Timestamp: time.Now(),
				Data:      contracts.TierReady{Name: "app", Duration: 100 * time.Millisecond, ServiceCount: 1},
			},
			traced: true,
			tiers:  2,
		},
		{
			name: "tier ready with invalid data keeps the transaction",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg: contracts.Message{
				Type: contracts.EventTierReady,
				Data: "invalid",
			},
			traced: true,
		},
		{
			name:   "tier ready without a transaction records nothing",
			before: func() *Tracer { return NewTracer(mockSubscriber) },
			msg: contracts.Message{
				Type:      contracts.EventTierReady,
				Timestamp: time.Now(),
				Data:      contracts.TierReady{Name: "app", Duration: 100 * time.Millisecond},
			},
		},
		{
			name: "watch triggered adds a span",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg: contracts.Message{
				Type: contracts.EventWatchTriggered,
				Data: contracts.WatchTriggered{Service: api},
			},
			traced: true,
		},
		{
			name:   "watch triggered without a transaction records nothing",
			before: func() *Tracer { return NewTracer(mockSubscriber) },
			msg: contracts.Message{
				Type: contracts.EventWatchTriggered,
				Data: contracts.WatchTriggered{Service: api},
			},
		},
		{
			name: "stop service adds a span",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg: contracts.Message{
				Type: contracts.CommandStopService,
				Data: api,
			},
			traced: true,
		},
		{
			name:   "stop service without a transaction records nothing",
			before: func() *Tracer { return NewTracer(mockSubscriber) },
			msg: contracts.Message{
				Type: contracts.CommandStopService,
				Data: api,
			},
		},
		{
			name: "restart service adds a span",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg: contracts.Message{
				Type: contracts.CommandRestartService,
				Data: api,
			},
			traced: true,
		},
		{
			name:   "restart service without a transaction records nothing",
			before: func() *Tracer { return NewTracer(mockSubscriber) },
			msg: contracts.Message{
				Type: contracts.CommandRestartService,
				Data: api,
			},
		},
		{
			name: "phase stopped finishes the transaction with a shutdown span",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg: contracts.Message{
				Type:      contracts.EventPhaseChanged,
				Timestamp: time.Now(),
				Data:      contracts.PhaseChanged{Phase: model.PhaseStopped, Duration: 500 * time.Millisecond},
			},
		},
		{
			name: "phase stopped without a duration finishes the transaction",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg: contracts.Message{
				Type:      contracts.EventPhaseChanged,
				Timestamp: time.Now(),
				Data:      contracts.PhaseChanged{Phase: model.PhaseStopped},
			},
		},
		{
			name: "a phase other than stopped keeps the transaction",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg: contracts.Message{
				Type:      contracts.EventPhaseChanged,
				Timestamp: time.Now(),
				Data:      contracts.PhaseChanged{Phase: model.PhaseRunning, Duration: 2 * time.Second},
			},
			traced: true,
		},
		{
			name: "phase changed with invalid data keeps the transaction",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg: contracts.Message{
				Type: contracts.EventPhaseChanged,
				Data: "invalid",
			},
			traced: true,
		},
		{
			name:   "phase changed without a transaction records nothing",
			before: func() *Tracer { return NewTracer(mockSubscriber) },
			msg: contracts.Message{
				Type:      contracts.EventPhaseChanged,
				Timestamp: time.Now(),
				Data:      contracts.PhaseChanged{Phase: model.PhaseStopped},
			},
		},
		{
			name: "an unhandled event keeps the transaction",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.handle(t.Context(), started)

				return tracer
			},
			msg: contracts.Message{
				Type: contracts.EventSignalReceived,
				Data: contracts.SignalReceived{Name: "SIGTERM"},
			},
			traced: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer := tt.before()

			tracer.handle(t.Context(), tt.msg)

			assert.Equal(t, tt.traced, tracer.trace != nil)
			assert.Len(t, tracer.tiers, tt.tiers)
		})
	}
}

func Test_Tracer_finish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	tracer := NewTracer(mockSubscriber)

	tests := []struct {
		name   string
		before func()
	}{
		{
			name: "finishes an open transaction",
			before: func() {
				tracer.trace = sentry.StartTransaction(t.Context(), "fuku run")
			},
		},
		{
			name:   "ignores a missing transaction",
			before: func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			tracer.finish(sentry.SpanStatusOK)

			assert.Nil(t, tracer.trace)
		})
	}
}

func Test_Tracer_tierPosition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	resolved := contracts.Message{
		Type:      contracts.EventProfileResolved,
		Timestamp: time.Now(),
		Data: contracts.ProfileResolved{
			Profile: "default",
			Tiers:   []model.Tier{{Name: "foundation"}, {Name: "app"}, {Name: "edge"}},
		},
	}

	tests := []struct {
		name   string
		before func() *Tracer
		tier   string
		index  int
		total  int
	}{
		{
			name: "a known tier is numbered from one",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.trace = sentry.StartTransaction(t.Context(), "fuku run")
				tracer.handle(t.Context(), resolved)

				return tracer
			},
			tier:  "app",
			index: 2,
			total: 3,
		},
		{
			name: "an unknown tier has no position",
			before: func() *Tracer {
				tracer := NewTracer(mockSubscriber)
				tracer.trace = sentry.StartTransaction(t.Context(), "fuku run")
				tracer.handle(t.Context(), resolved)

				return tracer
			},
			tier:  "missing",
			index: 0,
			total: 3,
		},
		{
			name:   "no tiers means no position and no total",
			before: func() *Tracer { return NewTracer(mockSubscriber) },
			tier:   "any",
			index:  0,
			total:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer := tt.before()

			index, total := tracer.tierPosition(tt.tier)

			assert.Equal(t, tt.index, index)
			assert.Equal(t, tt.total, total)
		})
	}
}

func Test_Tracer_TransactionSpan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTransport := NewMockTransport(ctrl)
	mockTransport.EXPECT().Configure(gomock.Any())

	client, err := sentry.NewClient(sentry.ClientOptions{Transport: mockTransport, EnableTracing: true, TracesSampleRate: 1.0})
	require.NoError(t, err)

	hub := sentry.NewHub(client, sentry.NewScope())
	ctx := sentry.SetHubOnContext(t.Context(), hub)

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	started := contracts.Message{
		Type:      contracts.EventCommandStarted,
		Timestamp: base,
		Data:      contracts.CommandStarted{Command: "run"},
	}
	stopped := contracts.Message{
		Type:      contracts.EventPhaseChanged,
		Timestamp: base.Add(9 * time.Minute),
		Data:      contracts.PhaseChanged{Phase: model.PhaseStopped},
	}

	var captured *sentry.Event

	capture := func(event *sentry.Event) { captured = event }

	mockTransport.EXPECT().SendEvent(gomock.Any()).Do(capture)

	tracer := NewTracer(nil)
	tracer.handle(ctx, started)

	tracer.handle(ctx, stopped)

	trace := captured.Contexts["trace"]
	status, hasStatus := trace["status"].(sentry.SpanStatus)
	_, hasOp := trace["op"]
	_, hasDescription := trace["description"]

	assert.Equal(t, "fuku run", captured.Transaction)
	assert.True(t, base.Equal(captured.StartTime))
	assert.True(t, hasStatus)
	assert.Equal(t, sentry.SpanStatusOK, status)
	assert.False(t, hasOp)
	assert.False(t, hasDescription)
}

func Test_Tracer_TimedSpans(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTransport := NewMockTransport(ctrl)
	mockTransport.EXPECT().Configure(gomock.Any())

	client, err := sentry.NewClient(sentry.ClientOptions{Transport: mockTransport, EnableTracing: true, TracesSampleRate: 1.0})
	require.NoError(t, err)

	hub := sentry.NewHub(client, sentry.NewScope())
	ctx := sentry.SetHubOnContext(t.Context(), hub)

	api := model.Service{ID: "test-id-api", Name: "api"}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	started := contracts.Message{
		Type:      contracts.EventCommandStarted,
		Timestamp: base,
		Data:      contracts.CommandStarted{Command: "run"},
	}
	resolved := contracts.Message{
		Type:      contracts.EventProfileResolved,
		Timestamp: base.Add(time.Minute),
		Data: contracts.ProfileResolved{
			Tiers:    []model.Tier{{Name: "app", Services: []*model.Service{&api}}},
			Duration: 5 * time.Second,
		},
	}
	preflightComplete := contracts.Message{
		Type:      contracts.EventPreflightComplete,
		Timestamp: base.Add(2 * time.Minute),
		Data:      contracts.PreflightComplete{Killed: 1, Duration: 4 * time.Second},
	}
	tierReady := contracts.Message{
		Type:      contracts.EventTierReady,
		Timestamp: base.Add(3 * time.Minute),
		Data:      contracts.TierReady{Name: "app", Duration: 10 * time.Second, ServiceCount: 1},
	}
	stoppedNoDuration := contracts.Message{
		Type:      contracts.EventPhaseChanged,
		Timestamp: base.Add(9 * time.Minute),
		Data:      contracts.PhaseChanged{Phase: model.PhaseStopped},
	}
	stoppedWithDuration := contracts.Message{
		Type:      contracts.EventPhaseChanged,
		Timestamp: base.Add(10 * time.Minute),
		Data:      contracts.PhaseChanged{Phase: model.PhaseStopped, Duration: 30 * time.Second},
	}

	var captured *sentry.Event

	capture := func(event *sentry.Event) { captured = event }
	withOp := func(op string) func(*sentry.Span) bool {
		return func(span *sentry.Span) bool { return span.Op == op }
	}

	tests := []struct {
		name        string
		before      func() *Tracer
		msg         contracts.Message
		op          string
		description string
		startTime   time.Time
	}{
		{
			name: "discovery span",
			before: func() *Tracer {
				mockTransport.EXPECT().SendEvent(gomock.Any()).Do(capture)

				tracer := NewTracer(nil)
				tracer.handle(ctx, started)
				tracer.handle(ctx, resolved)

				return tracer
			},
			msg:         stoppedNoDuration,
			op:          OpDiscovery,
			description: "",
			startTime:   resolved.Timestamp.Add(-5 * time.Second),
		},
		{
			name: "preflight span",
			before: func() *Tracer {
				mockTransport.EXPECT().SendEvent(gomock.Any()).Do(capture)

				tracer := NewTracer(nil)
				tracer.handle(ctx, started)
				tracer.handle(ctx, preflightComplete)

				return tracer
			},
			msg:         stoppedNoDuration,
			op:          OpPreflight,
			description: "",
			startTime:   preflightComplete.Timestamp.Add(-4 * time.Second),
		},
		{
			name: "tier_startup span",
			before: func() *Tracer {
				mockTransport.EXPECT().SendEvent(gomock.Any()).Do(capture)

				tracer := NewTracer(nil)
				tracer.handle(ctx, started)
				tracer.handle(ctx, resolved)
				tracer.handle(ctx, tierReady)

				return tracer
			},
			msg:         stoppedNoDuration,
			op:          OpTierStartup,
			description: "tier 1/1 (1 services)",
			startTime:   tierReady.Timestamp.Add(-10 * time.Second),
		},
		{
			name: "shutdown span",
			before: func() *Tracer {
				mockTransport.EXPECT().SendEvent(gomock.Any()).Do(capture)

				tracer := NewTracer(nil)
				tracer.handle(ctx, started)

				return tracer
			},
			msg:         stoppedWithDuration,
			op:          OpShutdown,
			description: "",
			startTime:   stoppedWithDuration.Timestamp.Add(-30 * time.Second),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer := tt.before()

			tracer.handle(ctx, tt.msg)

			idx := slices.IndexFunc(captured.Spans, withOp(tt.op))
			require.GreaterOrEqual(t, idx, 0)

			span := captured.Spans[idx]

			assert.Equal(t, tt.description, span.Description)
			assert.Equal(t, sentry.SpanStatusUndefined, span.Status)
			assert.True(t, tt.startTime.Equal(span.StartTime))
		})
	}
}

func Test_Tracer_MarkSpans(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTransport := NewMockTransport(ctrl)
	mockTransport.EXPECT().Configure(gomock.Any())

	client, err := sentry.NewClient(sentry.ClientOptions{Transport: mockTransport, EnableTracing: true, TracesSampleRate: 1.0})
	require.NoError(t, err)

	hub := sentry.NewHub(client, sentry.NewScope())
	ctx := sentry.SetHubOnContext(t.Context(), hub)

	api := model.Service{ID: "test-id-api", Name: "api"}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	started := contracts.Message{
		Type:      contracts.EventCommandStarted,
		Timestamp: base,
		Data:      contracts.CommandStarted{Command: "run"},
	}
	watchTriggered := contracts.Message{
		Type: contracts.EventWatchTriggered,
		Data: contracts.WatchTriggered{Service: api},
	}
	stopService := contracts.Message{
		Type: contracts.CommandStopService,
		Data: api,
	}
	restartService := contracts.Message{
		Type: contracts.CommandRestartService,
		Data: api,
	}
	stoppedNoDuration := contracts.Message{
		Type:      contracts.EventPhaseChanged,
		Timestamp: base.Add(9 * time.Minute),
		Data:      contracts.PhaseChanged{Phase: model.PhaseStopped},
	}

	var captured *sentry.Event

	capture := func(event *sentry.Event) { captured = event }
	withOp := func(op string) func(*sentry.Span) bool {
		return func(span *sentry.Span) bool { return span.Op == op }
	}

	tests := []struct {
		name        string
		before      func() *Tracer
		msg         contracts.Message
		op          string
		description string
	}{
		{
			name: "watch_restart mark",
			before: func() *Tracer {
				mockTransport.EXPECT().SendEvent(gomock.Any()).Do(capture)

				tracer := NewTracer(nil)
				tracer.handle(ctx, started)
				tracer.handle(ctx, watchTriggered)

				return tracer
			},
			msg: stoppedNoDuration,
			op:  OpWatchRestart,
		},
		{
			name: "service_stop mark",
			before: func() *Tracer {
				mockTransport.EXPECT().SendEvent(gomock.Any()).Do(capture)

				tracer := NewTracer(nil)
				tracer.handle(ctx, started)
				tracer.handle(ctx, stopService)

				return tracer
			},
			msg: stoppedNoDuration,
			op:  OpServiceStop,
		},
		{
			name: "service_restart mark",
			before: func() *Tracer {
				mockTransport.EXPECT().SendEvent(gomock.Any()).Do(capture)

				tracer := NewTracer(nil)
				tracer.handle(ctx, started)
				tracer.handle(ctx, restartService)

				return tracer
			},
			msg: stoppedNoDuration,
			op:  OpServiceRestart,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer := tt.before()

			tracer.handle(ctx, tt.msg)

			idx := slices.IndexFunc(captured.Spans, withOp(tt.op))
			require.GreaterOrEqual(t, idx, 0)

			span := captured.Spans[idx]

			assert.Equal(t, tt.description, span.Description)
			assert.Equal(t, sentry.SpanStatusUndefined, span.Status)
		})
	}
}
