package bus

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewBus(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	b := NewBus(cfg, nil, nil)

	assert.NotNil(t, b)
}

func Test_Bus_PublishSubscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	formatter := NewFormatter(logger.NewEventLogger())

	b := NewBus(cfg, formatter, mockLog)
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

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	formatter := NewFormatter(logger.NewEventLogger())

	b := NewBus(cfg, formatter, mockLog)
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
	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	b := NewBus(cfg, nil, nil)
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
	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	b := NewBus(cfg, nil, nil)

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

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	formatter := NewFormatter(logger.NewEventLogger())

	cfg := &config.Config{}
	cfg.Logs.Buffer = 1

	b := NewBus(cfg, formatter, mockLog)
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

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	formatter := NewFormatter(logger.NewEventLogger())

	b := NewBus(cfg, formatter, mockLog)
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

func Test_Bus_Publish_WithLoggerAndFormatter(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	log := logger.NewLoggerWithOutput(&config.Config{
		Logging: struct {
			Level  string `yaml:"level"`
			Format string `yaml:"format"`
		}{Level: "debug"},
	}, io.Discard)

	formatter := NewFormatter(logger.NewEventLogger())

	b := NewBus(cfg, formatter, log)
	defer b.Close()

	b.Subscribe(t.Context())

	b.Publish(Message{
		Type: EventServiceReady,
		Data: ServiceReady{ServiceEvent: ServiceEvent{Service: "api", Tier: "platform"}},
	})
}

func Test_Bus_Close_AlreadyClosed(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	b := NewBus(cfg, nil, nil)

	b.Close()
	b.Close()
}

func Test_NoOp_Methods(t *testing.T) {
	b := NoOp()

	b.Publish(Message{Type: EventPhaseChanged})
	b.Close()
}
