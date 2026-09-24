package contracts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_MessageType_Critical(t *testing.T) {
	tests := []struct {
		name     string
		msgType  MessageType
		expected string
		critical bool
	}{
		{
			name:     "command started",
			msgType:  EventCommandStarted,
			expected: "command_started",
			critical: false,
		},
		{
			name:     "phase changed",
			msgType:  EventPhaseChanged,
			expected: "phase_changed",
			critical: true,
		},
		{
			name:     "profile resolved",
			msgType:  EventProfileResolved,
			expected: "profile_resolved",
			critical: true,
		},
		{
			name:     "preflight started",
			msgType:  EventPreflightStarted,
			expected: "preflight_started",
			critical: true,
		},
		{
			name:     "preflight killed",
			msgType:  EventPreflightKilled,
			expected: "preflight_kill",
			critical: false,
		},
		{
			name:     "preflight complete",
			msgType:  EventPreflightComplete,
			expected: "preflight_complete",
			critical: true,
		},
		{
			name:     "tier starting",
			msgType:  EventTierStarting,
			expected: "tier_starting",
			critical: true,
		},
		{
			name:     "tier ready",
			msgType:  EventTierReady,
			expected: "tier_ready",
			critical: true,
		},
		{
			name:     "service starting",
			msgType:  EventServiceStarting,
			expected: "service_starting",
			critical: true,
		},
		{
			name:     "readiness complete",
			msgType:  EventReadinessComplete,
			expected: "readiness_complete",
			critical: false,
		},
		{
			name:     "service ready",
			msgType:  EventServiceReady,
			expected: "service_ready",
			critical: true,
		},
		{
			name:     "service failed",
			msgType:  EventServiceFailed,
			expected: "service_failed",
			critical: true,
		},
		{
			name:     "service stopping",
			msgType:  EventServiceStopping,
			expected: "service_stopping",
			critical: true,
		},
		{
			name:     "service stopped",
			msgType:  EventServiceStopped,
			expected: "service_stopped",
			critical: true,
		},
		{
			name:     "service restarting",
			msgType:  EventServiceRestarting,
			expected: "service_restarting",
			critical: true,
		},
		{
			name:     "signal received",
			msgType:  EventSignalReceived,
			expected: "signal",
			critical: true,
		},
		{
			name:     "watch triggered",
			msgType:  EventWatchTriggered,
			expected: "watch_triggered",
			critical: true,
		},
		{
			name:     "watch started",
			msgType:  EventWatchStarted,
			expected: "watch_started",
			critical: false,
		},
		{
			name:     "watch stopped",
			msgType:  EventWatchStopped,
			expected: "watch_stopped",
			critical: false,
		},
		{
			name:     "resource sampled",
			msgType:  EventResourceSampled,
			expected: "resource_sample",
			critical: false,
		},
		{
			name:     "service resources sampled",
			msgType:  EventServiceResourcesSampled,
			expected: "service_resources_sampled",
			critical: false,
		},
		{
			name:     "api started",
			msgType:  EventAPIStarted,
			expected: "api_started",
			critical: true,
		},
		{
			name:     "api stopped",
			msgType:  EventAPIStopped,
			expected: "api_stopped",
			critical: true,
		},
		{
			name:     "api requested",
			msgType:  EventAPIRequested,
			expected: "api_request",
			critical: false,
		},
		{
			name:     "update available",
			msgType:  EventUpdateAvailable,
			expected: "update_available",
			critical: false,
		},
		{
			name:     "snapshot changed",
			msgType:  EventSnapshotChanged,
			expected: "snapshot_changed",
			critical: false,
		},
		{
			name:     "start service",
			msgType:  CommandStartService,
			expected: "cmd_start_service",
			critical: true,
		},
		{
			name:     "stop service",
			msgType:  CommandStopService,
			expected: "cmd_stop_service",
			critical: true,
		},
		{
			name:     "restart service",
			msgType:  CommandRestartService,
			expected: "cmd_restart_service",
			critical: true,
		},
		{
			name:     "stop all",
			msgType:  CommandStopAll,
			expected: "cmd_stop_all",
			critical: true,
		},
	}

	assert.Len(t, critical, len(tests), "every message type must have a row")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, critical, tt.msgType)
			assert.Equal(t, tt.expected, string(tt.msgType))
			assert.Equal(t, tt.critical, tt.msgType.Critical())
		})
	}
}

func Test_MessageType_Critical_Unknown(t *testing.T) {
	assert.False(t, MessageType("unknown").Critical())
}
