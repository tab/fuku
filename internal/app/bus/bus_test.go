package bus

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/logs"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func newMockEvent(ctrl *gomock.Controller) logger.EventLogger {
	mock := logger.NewMockEventLogger(ctrl)
	mock.EXPECT().NewLogger(gomock.Any()).DoAndReturn(func(buf *bytes.Buffer) zerolog.Logger {
		return zerolog.New(buf)
	}).AnyTimes()

	return mock
}

func Test_NewBus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	mockServer := logs.NewMockServer(ctrl)
	mockEvent := newMockEvent(ctrl)

	b := NewBus(cfg, mockServer, mockEvent, nil)

	assert.NotNil(t, b)
}

func Test_Bus_PublishSubscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	mockEvent := newMockEvent(ctrl)

	b := NewBus(cfg, mockServer, mockEvent, mockLog)
	defer b.Close()

	ch := b.Subscribe(t.Context())

	b.Publish(Message{
		Type: EventServiceReady,
		Data: ServiceReady{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
	})

	select {
	case msg := <-ch:
		assert.Equal(t, EventServiceReady, msg.Type)
		data, ok := msg.Data.(ServiceReady)
		assert.True(t, ok)
		assert.Equal(t, "api", data.Service)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected message")
	}
}

func Test_Bus_MultipleSubscribers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	mockEvent := newMockEvent(ctrl)

	b := NewBus(cfg, mockServer, mockEvent, mockLog)
	defer b.Close()

	ch1 := b.Subscribe(t.Context())
	ch2 := b.Subscribe(t.Context())

	b.Publish(Message{Type: EventPhaseChanged, Data: PhaseChanged{Phase: PhaseRunning}})

	for _, ch := range []<-chan Message{ch1, ch2} {
		select {
		case msg := <-ch:
			assert.Equal(t, EventPhaseChanged, msg.Type)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Expected message on subscriber")
		}
	}
}

func Test_Bus_Unsubscribe_OnContextCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	mockServer := logs.NewMockServer(ctrl)
	mockEvent := newMockEvent(ctrl)

	b := NewBus(cfg, mockServer, mockEvent, nil)
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ch := b.Subscribe(ctx)

	cancel()

	select {
	case _, ok := <-ch:
		assert.False(t, ok, "Channel should be closed after context cancel")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Channel was not closed after context cancel")
	}
}

func Test_Bus_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	mockServer := logs.NewMockServer(ctrl)
	mockEvent := newMockEvent(ctrl)

	b := NewBus(cfg, mockServer, mockEvent, nil)

	ctx := context.Background()
	ch := b.Subscribe(ctx)

	b.Close()

	_, ok := <-ch
	assert.False(t, ok, "Channel should be closed")

	b.Publish(Message{Type: EventPhaseChanged})
}

func Test_Bus_CriticalMessage_BlockingSubscriber(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	mockEvent := newMockEvent(ctrl)

	cfg := &config.Config{}
	cfg.Logs.Buffer = 1

	b := NewBus(cfg, mockServer, mockEvent, mockLog)
	defer b.Close()

	ch := b.Subscribe(t.Context())

	b.Publish(Message{Type: EventPhaseChanged, Critical: false})

	b.Publish(Message{Type: EventServiceReady, Critical: true})

	received := 0
	timeout := time.After(100 * time.Millisecond)

loop:
	for {
		select {
		case <-ch:
			received++
			if received >= 2 {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	assert.GreaterOrEqual(t, received, 1)
}

func Test_Bus_Command(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	mockEvent := newMockEvent(ctrl)

	b := NewBus(cfg, mockServer, mockEvent, mockLog)
	defer b.Close()

	ch := b.Subscribe(t.Context())

	b.Publish(Message{
		Type: CommandStopService,
		Data: Payload{Name: "api"},
	})

	select {
	case msg := <-ch:
		assert.Equal(t, CommandStopService, msg.Type)
		data, ok := msg.Data.(Payload)
		assert.True(t, ok)
		assert.Equal(t, "api", data.Name)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected command message")
	}
}

func Test_NoOp(t *testing.T) {
	b := NoOp()

	assert.NotNil(t, b)

	ctx, cancel := context.WithCancel(context.Background())
	ch := b.Subscribe(ctx)

	b.Publish(Message{Type: EventPhaseChanged})

	select {
	case <-ch:
		t.Fatal("NoOp should not deliver messages")
	case <-time.After(10 * time.Millisecond):
	}

	cancel()

	select {
	case _, ok := <-ch:
		assert.False(t, ok, "Channel should be closed after context cancel")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Channel was not closed after context cancel")
	}

	b.Close()
}

func Test_Bus_Publish_WithLoggerAndBroadcaster(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	log := logger.NewLoggerWithOutput(&config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{Level: "debug"},
	}, io.Discard)

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast("fuku", gomock.Any()).Times(1)

	mockEvent := newMockEvent(ctrl)

	b := NewBus(cfg, mockServer, mockEvent, log)
	defer b.Close()

	b.Subscribe(t.Context())

	b.Publish(Message{
		Type: EventServiceReady,
		Data: ServiceReady{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
	})
}

func Test_Bus_Close_AlreadyClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	mockServer := logs.NewMockServer(ctrl)
	mockEvent := newMockEvent(ctrl)

	b := NewBus(cfg, mockServer, mockEvent, nil)

	b.Close()
	b.Close()
}

func Test_NoOp_Methods(t *testing.T) {
	b := NoOp()

	// These should not panic
	b.Publish(Message{Type: EventPhaseChanged})
	b.Close()
}

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
			name:     "Payload",
			msgType:  CommandStopService,
			data:     Payload{Name: "api"},
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
			data:     ServiceStarting{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}, PID: 123},
			contains: []string{"service_starting", "service=api", "tier=platform", "pid=123"},
		},
		{
			name:     "ReadinessComplete",
			msgType:  EventReadinessComplete,
			data:     ReadinessComplete{Service: "api", Type: "http", Duration: time.Second},
			contains: []string{"readiness_complete", "service=api", "type=http", "duration=1s"},
		},
		{
			name:     "ServiceReady",
			msgType:  EventServiceReady,
			data:     ServiceReady{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
			contains: []string{"service_ready", "service=api", "tier=platform"},
		},
		{
			name:     "ServiceFailed",
			msgType:  EventServiceFailed,
			data:     ServiceFailed{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}, Error: nil},
			contains: []string{"service_failed", "service=api", "tier=platform"},
		},
		{
			name:     "ServiceStopping",
			msgType:  EventServiceStopping,
			data:     ServiceStopping{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
			contains: []string{"service_stopping", "service=api", "tier=platform"},
		},
		{
			name:     "ServiceStopped",
			msgType:  EventServiceStopped,
			data:     ServiceStopped{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
			contains: []string{"service_stopped", "service=api", "tier=platform"},
		},
		{
			name:     "ServiceRestarting",
			msgType:  EventServiceRestarting,
			data:     ServiceRestarting{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
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
			data:     WatchTriggered{Service: "api", ChangedFiles: []string{"main.go"}},
			contains: []string{"watch_triggered", "service=api", "main.go"},
		},
		{
			name:     "ResourceSample",
			msgType:  EventResourceSample,
			data:     ResourceSample{CPU: 2.5, MEM: 64.0},
			contains: []string{"resource_sample", "cpu=2.5%", "mem=64.0MB"},
		},
		{
			name:     "Unknown",
			msgType:  "unknown",
			data:     struct{ Foo string }{Foo: "bar"},
			contains: []string{"unknown", "bar"},
		},
	}

	b := &bus{event: logger.NewEventLogger()}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.formatEvent(tt.msgType, tt.data)
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
				ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"},
				Error:        errors.New("address already in use"),
			},
			contains: []string{"service_failed", "service=api", "tier=platform", `"address already in use"`},
		},
		{
			name:    "Service name with spaces is quoted",
			msgType: EventServiceReady,
			data: ServiceReady{
				ServiceEvent: ServiceEvent{Service: "api worker", Tier: "platform"},
			},
			contains: []string{"service_ready", `"api worker"`, "tier=platform"},
		},
		{
			name:     "No quoting without spaces",
			msgType:  EventServiceReady,
			data:     ServiceReady{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
			contains: []string{"service_ready", "service=api", "tier=platform"},
		},
		{
			name:     "Slice preserves commas in values",
			msgType:  EventWatchTriggered,
			data:     WatchTriggered{Service: "api", ChangedFiles: []string{"main.go", "foo,bar.go"}},
			contains: []string{"watch_triggered", "service=api", "main.go", "foo,bar.go"},
		},
	}

	b := &bus{event: logger.NewEventLogger()}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.formatEvent(tt.msgType, tt.data)
			for _, s := range tt.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}
