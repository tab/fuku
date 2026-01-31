package bus

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	return cfg
}

func Test_New(t *testing.T) {
	b := New(testConfig(), nil)

	assert.NotNil(t, b)
}

func Test_Bus_PublishSubscribe(t *testing.T) {
	b := New(testConfig(), nil)
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := b.Subscribe(ctx)

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
	b := New(testConfig(), nil)
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch1 := b.Subscribe(ctx)
	ch2 := b.Subscribe(ctx)

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
	b := New(testConfig(), nil)
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ch := b.Subscribe(ctx)

	cancel()
	time.Sleep(10 * time.Millisecond)

	_, ok := <-ch
	assert.False(t, ok, "Channel should be closed after context cancel")
}

func Test_Bus_Close(t *testing.T) {
	b := New(testConfig(), nil)

	ctx := context.Background()
	ch := b.Subscribe(ctx)

	b.Close()

	_, ok := <-ch
	assert.False(t, ok, "Channel should be closed")

	b.Publish(Message{Type: EventPhaseChanged})
}

func Test_Bus_CriticalMessage_BlockingSubscriber(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logs.Buffer = 1

	b := New(cfg, nil)
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := b.Subscribe(ctx)

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
	b := New(testConfig(), nil)
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := b.Subscribe(ctx)

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
	time.Sleep(10 * time.Millisecond)

	_, ok := <-ch
	assert.False(t, ok)

	b.Close()
}

func Test_Bus_SetBroadcaster(t *testing.T) {
	b := New(testConfig(), nil)
	defer b.Close()

	b.SetBroadcaster(nil)
}

type mockBroadcaster struct {
	broadcasts []struct {
		service string
		message string
	}
}

func (m *mockBroadcaster) Broadcast(service, message string) {
	m.broadcasts = append(m.broadcasts, struct {
		service string
		message string
	}{service, message})
}

func Test_Bus_Publish_WithLoggerAndBroadcaster(t *testing.T) {
	cfg := testConfig()
	log := logger.NewLoggerWithOutput(&config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{Level: "debug"},
	}, io.Discard)

	b := New(cfg, log)
	defer b.Close()

	broadcaster := &mockBroadcaster{}
	b.SetBroadcaster(broadcaster)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b.Subscribe(ctx)

	b.Publish(Message{
		Type: EventServiceReady,
		Data: ServiceReady{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
	})

	assert.Len(t, broadcaster.broadcasts, 1)
	assert.Equal(t, "fuku", broadcaster.broadcasts[0].service)
}

func Test_Bus_Close_AlreadyClosed(t *testing.T) {
	b := New(testConfig(), nil)

	b.Close()
	b.Close() // Should not panic
}

func Test_NoOp_Methods(t *testing.T) {
	b := NoOp()

	// These should not panic
	b.Publish(Message{Type: EventPhaseChanged})
	b.SetBroadcaster(nil)
	b.Close()
}

func Test_FormatData(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		contains string
	}{
		{
			name:     "ProfileResolved",
			data:     ProfileResolved{Profile: "default"},
			contains: "default",
		},
		{
			name:     "PhaseChanged",
			data:     PhaseChanged{Phase: PhaseRunning},
			contains: "running",
		},
		{
			name:     "TierStarting",
			data:     TierStarting{Name: "platform", Index: 1, Total: 3},
			contains: "platform",
		},
		{
			name:     "Payload",
			data:     Payload{Name: "api"},
			contains: "api",
		},
		{
			name:     "ServiceStarting",
			data:     ServiceStarting{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}, PID: 123},
			contains: "api",
		},
		{
			name:     "ServiceReady",
			data:     ServiceReady{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
			contains: "api",
		},
		{
			name:     "ServiceFailed",
			data:     ServiceFailed{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}, Error: nil},
			contains: "api",
		},
		{
			name:     "ServiceStopping",
			data:     ServiceStopping{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
			contains: "api",
		},
		{
			name:     "ServiceStopped",
			data:     ServiceStopped{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
			contains: "api",
		},
		{
			name:     "ServiceRestarting",
			data:     ServiceRestarting{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
			contains: "api",
		},
		{
			name:     "Signal",
			data:     Signal{Name: "SIGTERM"},
			contains: "SIGTERM",
		},
		{
			name:     "WatchTriggered",
			data:     WatchTriggered{Service: "api", ChangedFiles: []string{"main.go"}},
			contains: "api",
		},
		{
			name:     "Unknown",
			data:     struct{ Foo string }{Foo: "bar"},
			contains: "bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatData(tt.data)
			assert.Contains(t, result, tt.contains)
		})
	}
}
