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

// queue is a Subscription over a plain channel
type queue chan contracts.Message

func (q queue) Messages() <-chan contracts.Message {
	return q
}

func Test_NewCollector(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	collector := NewCollector(mockSubscriber)

	assert.NotNil(t, collector)
	assert.Equal(t, mockSubscriber, collector.subscriber)
}

func Test_Collector_Subscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	collector := NewCollector(mockSubscriber)

	tests := []struct {
		name     string
		before   func()
		expected error
	}{
		{
			name: "registers the optional metrics subscription and handles its messages",
			before: func() {
				messages := make(queue, 1)
				messages <- contracts.Message{
					Type: contracts.EventServiceFailed,
					Data: contracts.ServiceFailed{
						ServiceEvent: contracts.ServiceEvent{Service: model.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
					},
				}

				close(messages)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "metrics"}).Return(messages, nil)
			},
		},
		{
			name: "returns the subscribe error",
			before: func() {
				mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "metrics"}).Return(nil, contracts.ErrBusClosed)
			},
			expected: contracts.ErrBusClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := collector.Subscribe(t.Context())

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Collector_Drain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	tests := []struct {
		name   string
		before func() *Collector
	}{
		{
			name: "a collector that never subscribed has nothing to drain",
			before: func() *Collector {
				return NewCollector(mockSubscriber)
			},
		},
		{
			name: "a subscribed collector drains once it handled its closed queue",
			before: func() *Collector {
				collector := NewCollector(mockSubscriber)

				messages := make(queue, 1)
				messages <- contracts.Message{Type: contracts.EventSnapshotChanged, Data: contracts.SnapshotChanged{}}

				close(messages)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "metrics"}).Return(messages, nil)

				require.NoError(t, collector.Subscribe(t.Context()))

				return collector
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := tt.before()

			err := collector.Drain(t.Context())

			require.NoError(t, err)
		})
	}
}

func Test_Collector_handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTransport := NewMockTransport(ctrl)

	mockTransport.EXPECT().Configure(gomock.Any())
	mockTransport.EXPECT().FlushWithContext(gomock.Any()).Return(true).AnyTimes()

	client, err := sentry.NewClient(sentry.ClientOptions{Transport: mockTransport})
	require.NoError(t, err)

	hub := sentry.NewHub(client, sentry.NewScope())
	ctx := sentry.SetHubOnContext(t.Context(), hub)

	collector := NewCollector(nil)

	api := model.Service{ID: "test-id-api", Name: "api"}

	emitted := func(expected ...string) gomock.Matcher {
		sameNames := func(event *sentry.Event) bool {
			names := make([]string, 0, len(event.Metrics))
			for _, metric := range event.Metrics {
				names = append(names, metric.Name)
			}

			return slices.Equal(expected, names)
		}

		return gomock.Cond(sameNames)
	}

	tests := []struct {
		name   string
		before func()
		msg    contracts.Message
	}{
		{
			name: "profile resolved",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricServiceCount, MetricTierCount, MetricDiscoveryDuration))
			},
			msg: contracts.Message{
				Type: contracts.EventProfileResolved,
				Data: contracts.ProfileResolved{
					Profile: "default",
					Tiers: []model.Tier{
						{Name: "foundation", Services: []*model.Service{{ID: "test-id-db", Name: "db"}, {ID: "test-id-cache", Name: "cache"}}},
						{Name: "platform", Services: []*model.Service{&api}},
					},
					Duration: 50 * time.Millisecond,
				},
			},
		},
		{
			name:   "profile resolved with invalid data",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventProfileResolved,
				Data: "invalid",
			},
		},
		{
			name: "tier ready",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricTierStartupDuration))
			},
			msg: contracts.Message{
				Type: contracts.EventTierReady,
				Data: contracts.TierReady{Name: "platform", Duration: 200 * time.Millisecond, ServiceCount: 3},
			},
		},
		{
			name:   "tier ready with invalid data",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventTierReady,
				Data: "invalid",
			},
		},
		{
			name: "readiness complete",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricReadinessDuration))
			},
			msg: contracts.Message{
				Type: contracts.EventReadinessComplete,
				Data: contracts.ReadinessComplete{Service: api, Type: model.ReadinessHTTP, Duration: 150 * time.Millisecond},
			},
		},
		{
			name:   "readiness complete with invalid data",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventReadinessComplete,
				Data: "invalid",
			},
		},
		{
			name: "service ready",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricServiceStartupDuration))
			},
			msg: contracts.Message{
				Type: contracts.EventServiceReady,
				Data: contracts.ServiceReady{
					ServiceEvent: contracts.ServiceEvent{Service: api, Tier: "platform"},
					Duration:     300 * time.Millisecond,
				},
			},
		},
		{
			name:   "service ready with invalid data",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventServiceReady,
				Data: "invalid",
			},
		},
		{
			name: "service failed",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricServiceFailed))
			},
			msg: contracts.Message{
				Type: contracts.EventServiceFailed,
				Data: contracts.ServiceFailed{ServiceEvent: contracts.ServiceEvent{Service: api, Tier: "platform"}},
			},
		},
		{
			name: "service restarting",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricServiceRestart))
			},
			msg: contracts.Message{
				Type: contracts.EventServiceRestarting,
				Data: contracts.ServiceRestarting{ServiceEvent: contracts.ServiceEvent{Service: api, Tier: "platform"}},
			},
		},
		{
			name: "watch triggered",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricWatchRestart))
			},
			msg: contracts.Message{
				Type: contracts.EventWatchTriggered,
				Data: contracts.WatchTriggered{Service: api, ChangedFiles: []string{"main.go"}},
			},
		},
		{
			name: "preflight complete",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricPreflightKilled, MetricPreflightDuration))
			},
			msg: contracts.Message{
				Type: contracts.EventPreflightComplete,
				Data: contracts.PreflightComplete{Killed: 2},
			},
		},
		{
			name:   "preflight complete with invalid data",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventPreflightComplete,
				Data: "invalid",
			},
		},
		{
			name: "service stopped unexpectedly",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricUnexpectedExit))
			},
			msg: contracts.Message{
				Type: contracts.EventServiceStopped,
				Data: contracts.ServiceStopped{ServiceEvent: contracts.ServiceEvent{Service: api, Tier: "platform"}, Unexpected: true},
			},
		},
		{
			name:   "service stopped on request",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventServiceStopped,
				Data: contracts.ServiceStopped{ServiceEvent: contracts.ServiceEvent{Service: api, Tier: "platform"}},
			},
		},
		{
			name:   "service stopped with invalid data",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventServiceStopped,
				Data: "invalid",
			},
		},
		{
			name: "phase running with a startup duration",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricStartupDuration))
			},
			msg: contracts.Message{
				Type: contracts.EventPhaseChanged,
				Data: contracts.PhaseChanged{Phase: model.PhaseRunning, Duration: 2 * time.Second, ServiceCount: 5},
			},
		},
		{
			name: "phase stopped with a shutdown duration",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricShutdownDuration))
			},
			msg: contracts.Message{
				Type: contracts.EventPhaseChanged,
				Data: contracts.PhaseChanged{Phase: model.PhaseStopped, Duration: 500 * time.Millisecond, ServiceCount: 4},
			},
		},
		{
			name:   "phase changed without a duration",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventPhaseChanged,
				Data: contracts.PhaseChanged{Phase: model.PhaseRunning},
			},
		},
		{
			name:   "phase changed with invalid data",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventPhaseChanged,
				Data: "invalid",
			},
		},
		{
			name: "command started",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricAppRun))
			},
			msg: contracts.Message{
				Type: contracts.EventCommandStarted,
				Data: contracts.CommandStarted{Command: "run", Profile: "default", UI: true},
			},
		},
		{
			name:   "command started with invalid data",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventCommandStarted,
				Data: "invalid",
			},
		},
		{
			name: "resource sampled",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricFukuCPU, MetricFukuMemory))
			},
			msg: contracts.Message{
				Type: contracts.EventResourceSampled,
				Data: contracts.ResourceSampled{CPU: 2.5, Memory: 64 * 1024 * 1024},
			},
		},
		{
			name:   "resource sampled with invalid data",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventResourceSampled,
				Data: "invalid",
			},
		},
		{
			name: "API started",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricAPIEnabled))
			},
			msg: contracts.Message{
				Type: contracts.EventAPIStarted,
				Data: contracts.APIStarted{Listen: "127.0.0.1:9876"},
			},
		},
		{
			name: "API stopped",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricAPIEnabled))
			},
			msg: contracts.Message{
				Type: contracts.EventAPIStopped,
				Data: contracts.APIStopped{},
			},
		},
		{
			name: "API request",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricAPIRequests, MetricAPIRequestDuration))
			},
			msg: contracts.Message{
				Type: contracts.EventAPIRequested,
				Data: contracts.APIRequested{Method: "GET", Path: "/api/v1/services/test-id-api", Status: 200, Duration: 5 * time.Millisecond},
			},
		},
		{
			name: "API request refused",
			before: func() {
				mockTransport.EXPECT().SendEvent(emitted(MetricAPIRequests, MetricAPIRequestDuration, MetricAPIAuthFailures))
			},
			msg: contracts.Message{
				Type: contracts.EventAPIRequested,
				Data: contracts.APIRequested{Method: "POST", Path: "/api/v1/services/test-id-api/start", Status: 401, Duration: time.Millisecond},
			},
		},
		{
			name:   "API request with invalid data",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventAPIRequested,
				Data: "invalid",
			},
		},
		{
			name:   "unhandled event",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventSignalReceived,
				Data: contracts.SignalReceived{Name: "SIGTERM"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			collector.handle(ctx, tt.msg)
			hub.Flush(flushTimeout)
		})
	}
}

func Test_normalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no UUID",
			input:    "/api/v1/status",
			expected: "/api/v1/status",
		},
		{
			name:     "service by ID",
			input:    "/api/v1/services/550e8400-e29b-41d4-a716-446655440000",
			expected: "/api/v1/services/:id",
		},
		{
			name:     "service action",
			input:    "/api/v1/services/550e8400-e29b-41d4-a716-446655440000/start",
			expected: "/api/v1/services/:id/start",
		},
		{
			name:     "services list",
			input:    "/api/v1/services",
			expected: "/api/v1/services",
		},
		{
			name:     "non-UUID ID",
			input:    "/api/v1/services/not-a-uuid",
			expected: "/api/v1/services/:id",
		},
		{
			name:     "uppercase UUID",
			input:    "/api/v1/services/550E8400-E29B-41D4-A716-446655440000/restart",
			expected: "/api/v1/services/:id/restart",
		},
		{
			name:     "other path untouched",
			input:    "/api/v1/live",
			expected: "/api/v1/live",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.input)

			assert.Equal(t, tt.expected, result)
		})
	}
}
