package relay

import (
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

type broadcastMsg struct {
	service string
	message string
}

type captureBroadcaster struct {
	mu       sync.Mutex
	messages []broadcastMsg
	notify   chan struct{}
}

func newCaptureBroadcaster() *captureBroadcaster {
	return &captureBroadcaster{
		notify: make(chan struct{}, 100),
	}
}

func (b *captureBroadcaster) Broadcast(service, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.messages = append(b.messages, broadcastMsg{service: service, message: message})

	select {
	case b.notify <- struct{}{}:
	default:
	}
}

func (b *captureBroadcaster) getMessages() []broadcastMsg {
	b.mu.Lock()
	defer b.mu.Unlock()

	result := make([]broadcastMsg, len(b.messages))
	copy(result, b.messages)

	return result
}

func (b *captureBroadcaster) waitForMessages(count int) []broadcastMsg {
	for {
		b.mu.Lock()
		n := len(b.messages)
		b.mu.Unlock()

		if n >= count {
			return b.getMessages()
		}

		<-b.notify
	}
}

func Test_NewBridge(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBroadcaster := NewMockBroadcaster(ctrl)
	formatter := bus.NewFormatter(logger.NewEventLogger())
	b := bus.NoOp()

	bridge := NewBridge(b, mockBroadcaster, formatter)

	assert.NotNil(t, bridge)
}

func Test_Bridge_StartStop(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Logs.Buffer = 10

	log := logger.NewLoggerWithOutput(cfg, io.Discard)
	formatter := bus.NewFormatter(logger.NewEventLogger())
	broadcaster := newCaptureBroadcaster()

	b := bus.NewBus(cfg, formatter, log)
	defer b.Close()

	bridge := NewBridge(b, broadcaster, formatter)
	bridge.Start()

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
		},
	})

	messages := broadcaster.waitForMessages(1)

	bridge.Stop()

	require.GreaterOrEqual(t, len(messages), 1)
	assert.Equal(t, config.AppName, messages[0].service)
	assert.Contains(t, messages[0].message, "service_ready")
}

func Test_Bridge_ForwardsMultipleEvents(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Logs.Buffer = 10

	log := logger.NewLoggerWithOutput(cfg, io.Discard)
	formatter := bus.NewFormatter(logger.NewEventLogger())
	broadcaster := newCaptureBroadcaster()

	b := bus.NewBus(cfg, formatter, log)
	defer b.Close()

	bridge := NewBridge(b, broadcaster, formatter)
	bridge.Start()

	b.Publish(bus.Message{
		Type: bus.EventServiceStarting,
		Data: bus.ServiceStarting{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
			PID:          1234,
		},
	})

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
		},
	})

	messages := broadcaster.waitForMessages(2)

	bridge.Stop()

	require.GreaterOrEqual(t, len(messages), 2)
	assert.Contains(t, messages[0].message, "service_starting")
	assert.Contains(t, messages[1].message, "service_ready")
}

func Test_Bridge_StopsCleanly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Logs.Buffer = 10

	log := logger.NewLoggerWithOutput(cfg, io.Discard)
	formatter := bus.NewFormatter(logger.NewEventLogger())
	broadcaster := newCaptureBroadcaster()

	b := bus.NewBus(cfg, formatter, log)
	defer b.Close()

	bridge := NewBridge(b, broadcaster, formatter)
	bridge.Start()
	bridge.Stop()
}

func Test_Bridge_Stop_NilCancel(t *testing.T) {
	bridge := &Bridge{}

	bridge.Stop()
}
