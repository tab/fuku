package eventlog

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Formatter_Format(t *testing.T) {
	service := model.Service{ID: "test-id-api", Name: "api"}
	event := contracts.ServiceEvent{Service: service, Tier: "platform"}

	tests := []struct {
		name    string
		msgType contracts.MessageType
		data    any
		want    string
	}{
		{
			name:    "command started",
			msgType: contracts.EventCommandStarted,
			data:    contracts.CommandStarted{Command: "run", Profile: "default", UI: true},
			want:    "command_started command=run profile=default ui=true",
		},
		{
			name:    "phase changed",
			msgType: contracts.EventPhaseChanged,
			data:    contracts.PhaseChanged{Phase: model.PhaseRunning, Duration: time.Second, ServiceCount: 3},
			want:    "phase_changed duration=1s phase=running services=3",
		},
		{
			name:    "profile resolved",
			msgType: contracts.EventProfileResolved,
			data:    contracts.ProfileResolved{Profile: "default", Duration: time.Second},
			want:    "profile_resolved profile=default",
		},
		{
			name:    "preflight started",
			msgType: contracts.EventPreflightStarted,
			data:    contracts.PreflightStarted{Services: []string{"api", "db"}},
			want:    `preflight_started services=["api","db"]`,
		},
		{
			name:    "preflight killed",
			msgType: contracts.EventPreflightKilled,
			data:    contracts.PreflightKilled{Service: "api", PID: 1234, Name: "node"},
			want:    "preflight_kill name=node pid=1234 service=api",
		},
		{
			name:    "preflight complete",
			msgType: contracts.EventPreflightComplete,
			data:    contracts.PreflightComplete{Killed: 3, Duration: 2 * time.Second},
			want:    "preflight_complete duration=2s killed=3",
		},
		{
			name:    "tier starting",
			msgType: contracts.EventTierStarting,
			data:    contracts.TierStarting{Name: "platform"},
			want:    "tier_starting tier=platform",
		},
		{
			name:    "tier ready",
			msgType: contracts.EventTierReady,
			data:    contracts.TierReady{Name: "platform", Duration: time.Second, ServiceCount: 3},
			want:    "tier_ready duration=1s services=3 tier=platform",
		},
		{
			name:    "service starting",
			msgType: contracts.EventServiceStarting,
			data:    contracts.ServiceStarting{ServiceEvent: event, Attempt: 2, PID: 123},
			want:    "service_starting attempt=2 id=test-id-api pid=123 service=api tier=platform",
		},
		{
			name:    "readiness complete",
			msgType: contracts.EventReadinessComplete,
			data:    contracts.ReadinessComplete{Service: service, Type: "http", Duration: time.Second},
			want:    "readiness_complete duration=1s id=test-id-api service=api type=http",
		},
		{
			name:    "service ready",
			msgType: contracts.EventServiceReady,
			data:    contracts.ServiceReady{ServiceEvent: event, PID: 123, Duration: time.Second},
			want:    "service_ready id=test-id-api service=api tier=platform",
		},
		{
			name:    "service failed with error",
			msgType: contracts.EventServiceFailed,
			data:    contracts.ServiceFailed{ServiceEvent: event, Error: errors.New("address already in use")},
			want:    `service_failed error="address already in use" id=test-id-api service=api tier=platform`,
		},
		{
			name:    "service failed without error",
			msgType: contracts.EventServiceFailed,
			data:    contracts.ServiceFailed{ServiceEvent: event},
			want:    "service_failed id=test-id-api service=api tier=platform",
		},
		{
			name:    "service stopping",
			msgType: contracts.EventServiceStopping,
			data:    contracts.ServiceStopping{ServiceEvent: event},
			want:    "service_stopping id=test-id-api service=api tier=platform",
		},
		{
			name:    "service stopped",
			msgType: contracts.EventServiceStopped,
			data:    contracts.ServiceStopped{ServiceEvent: event, Unexpected: true},
			want:    "service_stopped id=test-id-api service=api tier=platform",
		},
		{
			name:    "service restarting",
			msgType: contracts.EventServiceRestarting,
			data:    contracts.ServiceRestarting{ServiceEvent: event},
			want:    "service_restarting id=test-id-api service=api tier=platform",
		},
		{
			name:    "signal received",
			msgType: contracts.EventSignalReceived,
			data:    contracts.SignalReceived{Name: "SIGTERM"},
			want:    "signal signal=SIGTERM",
		},
		{
			name:    "watch triggered",
			msgType: contracts.EventWatchTriggered,
			data:    contracts.WatchTriggered{Service: service, ChangedFiles: []string{"main.go", "foo,bar.go"}},
			want:    `watch_triggered files=["main.go","foo,bar.go"] id=test-id-api service=api`,
		},
		{
			name:    "watch started",
			msgType: contracts.EventWatchStarted,
			data:    contracts.WatchStarted{Service: service},
			want:    "watch_started id=test-id-api name=api",
		},
		{
			name:    "watch stopped",
			msgType: contracts.EventWatchStopped,
			data:    contracts.WatchStopped{Service: service},
			want:    "watch_stopped id=test-id-api name=api",
		},
		{
			name:    "resource sampled",
			msgType: contracts.EventResourceSampled,
			data:    contracts.ResourceSampled{CPU: 2.5, MEM: 64.0},
			want:    "resource_sample cpu=2.5% mem=64.0MB",
		},
		{
			name:    "api started",
			msgType: contracts.EventAPIStarted,
			data:    contracts.APIStarted{Listen: "127.0.0.1:8080"},
			want:    "api_started listen=127.0.0.1:8080",
		},
		{
			name:    "api stopped",
			msgType: contracts.EventAPIStopped,
			data:    contracts.APIStopped{},
			want:    "api_stopped",
		},
		{
			name:    "api requested",
			msgType: contracts.EventAPIRequested,
			data:    contracts.APIRequested{Method: "GET", Path: "/api/v1/status", Status: 200, Duration: 5 * time.Millisecond},
			want:    "api_request duration=5ms method=GET path=/api/v1/status status=200",
		},
		{
			name:    "update available",
			msgType: contracts.EventUpdateAvailable,
			data:    contracts.UpdateAvailable{Version: "v0.20.0"},
			want:    "update_available version=v0.20.0",
		},
		{
			name:    "start service command",
			msgType: contracts.CommandStartService,
			data:    service,
			want:    "cmd_start_service id=test-id-api name=api",
		},
		{
			name:    "stop service command",
			msgType: contracts.CommandStopService,
			data:    service,
			want:    "cmd_stop_service id=test-id-api name=api",
		},
		{
			name:    "restart service command",
			msgType: contracts.CommandRestartService,
			data:    service,
			want:    "cmd_restart_service id=test-id-api name=api",
		},
		{
			name:    "stop all command",
			msgType: contracts.CommandStopAll,
			data:    nil,
			want:    "cmd_stop_all",
		},
		{
			name:    "service name with spaces is quoted",
			msgType: contracts.EventServiceReady,
			data:    contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: model.Service{ID: "test-id-api", Name: "api worker"}, Tier: "platform"}},
			want:    `service_ready id=test-id-api service="api worker" tier=platform`,
		},
		{
			name:    "unknown payload",
			msgType: "unknown",
			data:    struct{ Foo string }{Foo: "bar"},
			want:    `unknown data={"Foo":"bar"}`,
		},
	}

	formatter := NewFormatter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatter.Format(tt.msgType, tt.data))
		})
	}
}
