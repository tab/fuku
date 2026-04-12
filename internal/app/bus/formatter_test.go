package bus

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/config/logger"
)

func Test_FormatEvent(t *testing.T) {
	tests := []struct {
		name     string
		msgType  MessageType
		data     any
		contains []string
	}{
		{
			name:     "CommandStarted",
			msgType:  EventCommandStarted,
			data:     CommandStarted{Command: "run", Profile: "default", UI: true},
			contains: []string{"command_started", "command=run", "profile=default", "ui=true"},
		},
		{
			name:     "PhaseChanged",
			msgType:  EventPhaseChanged,
			data:     PhaseChanged{Phase: PhaseRunning, Duration: time.Second, ServiceCount: 3},
			contains: []string{"phase_changed", "phase=running", "duration=1s", "services=3"},
		},
		{
			name:     "ProfileResolved",
			msgType:  EventProfileResolved,
			data:     ProfileResolved{Profile: "default"},
			contains: []string{"profile_resolved", "profile=default"},
		},
		{
			name:     "PreflightStarted",
			msgType:  EventPreflightStarted,
			data:     PreflightStarted{Services: []string{"api", "db"}},
			contains: []string{"preflight_started", "api", "db"},
		},
		{
			name:     "PreflightKill",
			msgType:  EventPreflightKill,
			data:     PreflightKill{Service: "api", PID: 1234, Name: "node"},
			contains: []string{"preflight_kill", "service=api", "pid=1234", "name=node"},
		},
		{
			name:     "PreflightComplete",
			msgType:  EventPreflightComplete,
			data:     PreflightComplete{Killed: 3},
			contains: []string{"preflight_complete", "killed=3"},
		},
		{
			name:     "TierStarting",
			msgType:  EventTierStarting,
			data:     TierStarting{Name: "platform"},
			contains: []string{"tier_starting", "tier=platform"},
		},
		{
			name:     "Service",
			msgType:  CommandStopService,
			data:     Service{ID: "test-id-api", Name: "api"},
			contains: []string{"cmd_stop_service", "name=api"},
		},
		{
			name:     "TierReady",
			msgType:  EventTierReady,
			data:     TierReady{Name: "platform", Duration: time.Second, ServiceCount: 3},
			contains: []string{"tier_ready", "tier=platform", "duration=1s", "services=3"},
		},
		{
			name:     "ServiceStarting",
			msgType:  EventServiceStarting,
			data:     ServiceStarting{ServiceEvent: ServiceEvent{Service: Service{ID: "test-id-api", Name: "api"}, Tier: "platform"}, PID: 123},
			contains: []string{"service_starting", "service=api", "tier=platform", "pid=123"},
		},
		{
			name:     "ReadinessComplete",
			msgType:  EventReadinessComplete,
			data:     ReadinessComplete{Service: Service{ID: "test-id-api", Name: "api"}, Type: "http", Duration: time.Second},
			contains: []string{"readiness_complete", "service=api", "type=http", "duration=1s"},
		},
		{
			name:     "ServiceReady",
			msgType:  EventServiceReady,
			data:     ServiceReady{ServiceEvent: ServiceEvent{Service: Service{ID: "test-id-api", Name: "api"}, Tier: "platform"}},
			contains: []string{"service_ready", "service=api", "tier=platform"},
		},
		{
			name:     "ServiceFailed",
			msgType:  EventServiceFailed,
			data:     ServiceFailed{ServiceEvent: ServiceEvent{Service: Service{ID: "test-id-api", Name: "api"}, Tier: "platform"}, Error: nil},
			contains: []string{"service_failed", "service=api", "tier=platform"},
		},
		{
			name:     "ServiceStopping",
			msgType:  EventServiceStopping,
			data:     ServiceStopping{ServiceEvent: ServiceEvent{Service: Service{ID: "test-id-api", Name: "api"}, Tier: "platform"}},
			contains: []string{"service_stopping", "service=api", "tier=platform"},
		},
		{
			name:     "ServiceStopped",
			msgType:  EventServiceStopped,
			data:     ServiceStopped{ServiceEvent: ServiceEvent{Service: Service{ID: "test-id-api", Name: "api"}, Tier: "platform"}},
			contains: []string{"service_stopped", "service=api", "tier=platform"},
		},
		{
			name:     "ServiceRestarting",
			msgType:  EventServiceRestarting,
			data:     ServiceRestarting{ServiceEvent: ServiceEvent{Service: Service{ID: "test-id-api", Name: "api"}, Tier: "platform"}},
			contains: []string{"service_restarting", "service=api", "tier=platform"},
		},
		{
			name:     "Signal",
			msgType:  EventSignal,
			data:     Signal{Name: "SIGTERM"},
			contains: []string{"signal", "signal=SIGTERM"},
		},
		{
			name:     "WatchTriggered",
			msgType:  EventWatchTriggered,
			data:     WatchTriggered{Service: Service{ID: "test-id-api", Name: "api"}, ChangedFiles: []string{"main.go"}},
			contains: []string{"watch_triggered", "service=api", "main.go"},
		},
		{
			name:     "ResourceSample",
			msgType:  EventResourceSample,
			data:     ResourceSample{CPU: 2.5, MEM: 64.0},
			contains: []string{"resource_sample", "cpu=2.5%", "mem=64.0MB"},
		},
		{
			name:    "ServiceMetricsBatch",
			msgType: EventServiceMetrics,
			data: ServiceMetricsBatch{Samples: []ServiceMetrics{
				{Service: Service{ID: "test-id-api", Name: "api"}, CPU: 3.2, Memory: 67108864},
				{Service: Service{ID: "test-id-web", Name: "web"}, CPU: 1.5, Memory: 33554432},
			}},
			contains: []string{"service_metrics", "services=2"},
		},
		{
			name:     "APIRequest",
			msgType:  EventAPIRequest,
			data:     APIRequest{Method: "GET", Path: "/api/v1/status", Status: 200, Duration: 5 * time.Millisecond},
			contains: []string{"api_request", "method=GET", "path=/api/v1/status", "status=200"},
		},
		{
			name:     "Unknown",
			msgType:  "unknown",
			data:     struct{ Foo string }{Foo: "bar"},
			contains: []string{"unknown", "bar"},
		},
	}

	f := NewFormatter(logger.NewEventLogger())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.Format(tt.msgType, tt.data)
			for _, s := range tt.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}

func Test_FormatEvent_QuotesAndSlices(t *testing.T) {
	tests := []struct {
		name     string
		msgType  MessageType
		data     any
		contains []string
	}{
		{
			name:    "Error with spaces is quoted",
			msgType: EventServiceFailed,
			data: ServiceFailed{
				ServiceEvent: ServiceEvent{Service: Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
				Error:        errors.New("address already in use"),
			},
			contains: []string{"service_failed", "service=api", "tier=platform", `"address already in use"`},
		},
		{
			name:    "Service name with spaces is quoted",
			msgType: EventServiceReady,
			data: ServiceReady{
				ServiceEvent: ServiceEvent{Service: Service{ID: "test-id-api", Name: "api worker"}, Tier: "platform"},
			},
			contains: []string{"service_ready", `"api worker"`, "tier=platform"},
		},
		{
			name:     "No quoting without spaces",
			msgType:  EventServiceReady,
			data:     ServiceReady{ServiceEvent: ServiceEvent{Service: Service{ID: "test-id-api", Name: "api"}, Tier: "platform"}},
			contains: []string{"service_ready", "service=api", "tier=platform"},
		},
		{
			name:     "Slice preserves commas in values",
			msgType:  EventWatchTriggered,
			data:     WatchTriggered{Service: Service{ID: "test-id-api", Name: "api"}, ChangedFiles: []string{"main.go", "foo,bar.go"}},
			contains: []string{"watch_triggered", "service=api", "main.go", "foo,bar.go"},
		},
	}

	f := NewFormatter(logger.NewEventLogger())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.Format(tt.msgType, tt.data)
			for _, s := range tt.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}
