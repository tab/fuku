package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
)

func Test_NewCollector(t *testing.T) {
	b := bus.NoOp()
	c := NewCollector(b)

	assert.NotNil(t, c)
}

func Test_Collector_Run_StopsOnContextCancel(t *testing.T) {
	c := NewCollector(bus.NoOp())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})

	go func() {
		c.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("collector did not stop after context cancel")
	}
}

func Test_Collector_Run_HandlesMessagesAndChannelClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ch := make(chan bus.Message, 10)
	mockBus := bus.NewMockBus(ctrl)
	mockBus.EXPECT().Subscribe(gomock.Any()).Return((<-chan bus.Message)(ch))

	c := NewCollector(mockBus)

	ch <- bus.Message{
		Type: bus.EventServiceFailed,
		Data: bus.ServiceFailed{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
		},
	}

	close(ch)

	done := make(chan struct{})

	go func() {
		c.Run(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("collector did not stop after channel close")
	}
}

func Test_Handle_ProfileResolved(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Profile: "default",
			Tiers: []bus.Tier{
				{Name: "foundation", Services: []bus.Service{{ID: "test-id-db", Name: "db"}, {ID: "test-id-cache", Name: "cache"}}},
				{Name: "platform", Services: []bus.Service{{ID: "test-id-api", Name: "api"}}},
			},
			Duration: 50 * time.Millisecond,
		},
	})
}

func Test_Handle_ProfileResolved_InvalidData(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventProfileResolved,
		Data: "invalid",
	})
}

func Test_Handle_TierReady(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventTierReady,
		Data: bus.TierReady{
			Name:         "platform",
			Duration:     200 * time.Millisecond,
			ServiceCount: 3,
		},
	})
}

func Test_Handle_TierReady_InvalidData(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventTierReady,
		Data: "invalid",
	})
}

func Test_Handle_ReadinessComplete(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventReadinessComplete,
		Data: bus.ReadinessComplete{
			Service:  bus.Service{ID: "test-id-api", Name: "api"},
			Type:     "http",
			Duration: 50 * time.Millisecond,
		},
	})
}

func Test_Handle_ReadinessComplete_InvalidData(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventReadinessComplete,
		Data: "invalid",
	})
}

func Test_Handle_ServiceReady(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
			Duration:     100 * time.Millisecond,
		},
	})
}

func Test_Handle_ServiceReady_InvalidData(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventServiceReady,
		Data: "invalid",
	})
}

func Test_Handle_ServiceFailed(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventServiceFailed,
		Data: bus.ServiceFailed{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
			Error:        assert.AnError,
		},
	})
}

func Test_Handle_ServiceRestarting(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventServiceRestarting,
		Data: bus.ServiceRestarting{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
		},
	})
}

func Test_Handle_WatchTriggered(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventWatchTriggered,
		Data: bus.WatchTriggered{
			Service:      bus.Service{ID: "test-id-api", Name: "api"},
			ChangedFiles: []string{"main.go"},
		},
	})
}

func Test_Handle_PreflightComplete(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventPreflightComplete,
		Data: bus.PreflightComplete{Killed: 2},
	})
}

func Test_Handle_PreflightComplete_InvalidData(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventPreflightComplete,
		Data: "invalid",
	})
}

func Test_Handle_ServiceStopped_Unexpected(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventServiceStopped,
		Data: bus.ServiceStopped{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
			Unexpected:   true,
		},
	})
}

func Test_Handle_ServiceStopped_Expected(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventServiceStopped,
		Data: bus.ServiceStopped{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
			Unexpected:   false,
		},
	})
}

func Test_Handle_ServiceStopped_InvalidData(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventServiceStopped,
		Data: "invalid",
	})
}

func Test_Handle_StartupDuration(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventPhaseChanged,
		Data: bus.PhaseChanged{
			Phase:        bus.PhaseRunning,
			Duration:     2 * time.Second,
			ServiceCount: 5,
		},
	})
}

func Test_Handle_ShutdownDuration(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventPhaseChanged,
		Data: bus.PhaseChanged{
			Phase:        bus.PhaseStopped,
			Duration:     500 * time.Millisecond,
			ServiceCount: 4,
		},
	})
}

func Test_Handle_PhaseChanged_NoDuration(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventPhaseChanged,
		Data: bus.PhaseChanged{Phase: bus.PhaseRunning},
	})
}

func Test_Handle_PhaseChanged_InvalidData(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventPhaseChanged,
		Data: "invalid",
	})
}

func Test_Handle_CommandStarted(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventCommandStarted,
		Data: bus.CommandStarted{
			Command: "run",
			Profile: "default",
			UI:      true,
		},
	})
}

func Test_Handle_CommandStarted_InvalidData(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventCommandStarted,
		Data: "invalid",
	})
}

func Test_Handle_ResourceSample(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventResourceSample,
		Data: bus.ResourceSample{
			CPU: 2.5,
			MEM: 64.0,
		},
	})
}

func Test_Handle_ResourceSample_InvalidData(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventResourceSample,
		Data: "invalid",
	})
}

func Test_Handle_UnhandledEvent(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventSignal,
		Data: bus.Signal{Name: "SIGTERM"},
	})
}

func Test_NormalizePath(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "no UUID",
			input:  "/api/v1/status",
			expect: "/api/v1/status",
		},
		{
			name:   "service by ID",
			input:  "/api/v1/services/550e8400-e29b-41d4-a716-446655440000",
			expect: "/api/v1/services/:id",
		},
		{
			name:   "service action",
			input:  "/api/v1/services/550e8400-e29b-41d4-a716-446655440000/start",
			expect: "/api/v1/services/:id/start",
		},
		{
			name:   "services list",
			input:  "/api/v1/services",
			expect: "/api/v1/services",
		},
		{
			name:   "non-UUID ID",
			input:  "/api/v1/services/not-a-uuid",
			expect: "/api/v1/services/:id",
		},
		{
			name:   "uppercase UUID",
			input:  "/api/v1/services/550E8400-E29B-41D4-A716-446655440000/restart",
			expect: "/api/v1/services/:id/restart",
		},
		{
			name:   "other path untouched",
			input:  "/api/v1/live",
			expect: "/api/v1/live",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, normalizePath(tt.input))
		})
	}
}

func Test_Handle_APIStarted(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventAPIStarted,
		Data: bus.APIStarted{Listen: "127.0.0.1:9876"},
	})
}

func Test_Handle_APIStopped(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventAPIStopped,
		Data: bus.APIStopped{},
	})
}

func Test_Handle_APIRequest(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventAPIRequest,
		Data: bus.APIRequest{
			Method:   "GET",
			Path:     "/api/v1/status",
			Status:   200,
			Duration: 5 * time.Millisecond,
		},
	})
}

func Test_Handle_APIRequest_AuthFailure(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventAPIRequest,
		Data: bus.APIRequest{
			Method:   "GET",
			Path:     "/api/v1/status",
			Status:   401,
			Duration: 1 * time.Millisecond,
		},
	})
}

func Test_Handle_APIRequest_InvalidData(t *testing.T) {
	c := &collector{}
	ctx := context.Background()

	c.handle(ctx, bus.Message{
		Type: bus.EventAPIRequest,
		Data: "invalid",
	})
}
