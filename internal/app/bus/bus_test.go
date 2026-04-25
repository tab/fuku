package bus

import (
	"context"
	"io"
	"sync"
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
		Data: ServiceReady{ServiceEvent: ServiceEvent{Service: Service{ID: "test-id-api", Name: "api"}, Tier: "platform"}},
	})

	select {
	case msg := <-ch:
		assert.Equal(t, EventServiceReady, msg.Type)
		assert.Equal(t, uint64(1), msg.Seq)
		data, ok := msg.Data.(ServiceReady)
		assert.True(t, ok)
		assert.Equal(t, "api", data.Service.Name)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected message")
	}
}

func Test_Bus_SeqIsMonotonic(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	b := NewBus(cfg, nil, nil)
	defer b.Close()

	ch := b.Subscribe(t.Context())

	for i := range 5 {
		b.Publish(Message{
			Type: EventServiceStarting,
			Data: ServiceStarting{ServiceEvent: ServiceEvent{Service: Service{ID: "api"}, Tier: "default"}, PID: i},
		})
	}

	var lastSeq uint64

	for range 5 {
		msg := <-ch
		assert.Greater(t, msg.Seq, lastSeq)
		lastSeq = msg.Seq
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
		Data: Service{Name: "api"},
	})

	select {
	case msg := <-ch:
		assert.Equal(t, CommandStopService, msg.Type)
		data, ok := msg.Data.(Service)
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
		Data: ServiceReady{ServiceEvent: ServiceEvent{Service: Service{ID: "test-id-api", Name: "api"}, Tier: "platform"}},
	})
}

func Test_ServiceEvent_ServiceName(t *testing.T) {
	e := ServiceEvent{Service: Service{ID: "test-id-api", Name: "api"}, Tier: "platform"}
	assert.Equal(t, "api", e.ServiceName())
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

func Test_Bus_CriticalFIFO_PublishOrderPreserved(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logs.Buffer = 1

	b := NewBus(cfg, nil, nil)
	defer b.Close()

	ch := b.Subscribe(t.Context())

	events := []Message{
		{Type: EventServiceStarting, Critical: true},
		{Type: EventServiceReady, Critical: true},
		{Type: EventServiceStopping, Critical: true},
		{Type: EventServiceStopped, Critical: true},
	}

	for _, e := range events {
		b.Publish(e)
	}

	received := make([]MessageType, 0, len(events))
	timeout := time.After(time.Second)

	for range events {
		select {
		case msg := <-ch:
			received = append(received, msg.Type)
		case <-timeout:
			t.Fatalf("timed out after receiving %d/%d messages", len(received), len(events))
		}
	}

	expected := []MessageType{
		EventServiceStarting,
		EventServiceReady,
		EventServiceStopping,
		EventServiceStopped,
	}

	assert.Equal(t, expected, received, "critical messages must be received in exact publish order")
}

func Test_Bus_Subscriber_CloseWithOverflow_NoPanic(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logs.Buffer = 1

	b := NewBus(cfg, nil, nil)

	ch := b.Subscribe(t.Context())

	b.Publish(Message{Type: EventPhaseChanged})

	b.Publish(Message{Type: EventServiceStarting, Critical: true})
	b.Publish(Message{Type: EventServiceReady, Critical: true})

	b.Close()

	drained := 0

	for range ch {
		drained++
	}

	assert.GreaterOrEqual(t, drained, 1, "should have received at least the buffered message")
}

func Test_Bus_Unsubscribe_WithOverflow_NoPanic(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logs.Buffer = 1

	b := NewBus(cfg, nil, nil)
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ch := b.Subscribe(ctx)

	b.Publish(Message{Type: EventPhaseChanged})

	b.Publish(Message{Type: EventServiceStarting, Critical: true})

	cancel()

	closed := false
	timeout := time.After(time.Second)

loop:
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				closed = true

				break loop
			}
		case <-timeout:
			break loop
		}
	}

	assert.True(t, closed, "channel should close after context cancel with overflow")
}

func Test_Bus_ConcurrentPublish_PreservesSubscriberOrder(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logs.Buffer = 1000

	b := NewBus(cfg, nil, nil)
	defer b.Close()

	ch := b.Subscribe(t.Context())

	const (
		publishers   = 4
		perPublisher = 50
	)

	total := publishers * perPublisher

	var wg sync.WaitGroup

	for range publishers {
		wg.Go(func() {
			for range perPublisher {
				b.Publish(Message{Type: EventResourceSample})
			}
		})
	}

	wg.Wait()

	var lastSeq uint64

	timeout := time.After(time.Second)

	for range total {
		select {
		case msg := <-ch:
			assert.Greater(t, msg.Seq, lastSeq, "seq must be strictly increasing")
			lastSeq = msg.Seq
		case <-timeout:
			t.Fatal("timed out waiting for messages")
		}
	}
}

func Test_Bus_ConcurrentLifecyclePublish_NoInversion(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logs.Buffer = 100

	b := NewBus(cfg, nil, nil)
	defer b.Close()

	ch := b.Subscribe(t.Context())

	const (
		goroutines   = 2
		perGoroutine = 25
	)

	var wg sync.WaitGroup

	for range goroutines {
		wg.Go(func() {
			for range perGoroutine {
				b.Publish(Message{
					Type:     EventServiceStarting,
					Critical: true,
				})
			}
		})
	}

	wg.Wait()

	total := goroutines * perGoroutine

	var lastSeq uint64

	timeout := time.After(time.Second)

	for range total {
		select {
		case msg := <-ch:
			assert.Greater(t, msg.Seq, lastSeq, "lifecycle seq must not invert")
			lastSeq = msg.Seq
		case <-timeout:
			t.Fatal("timed out waiting for lifecycle messages")
		}
	}
}

func Test_Bus_CriticalOverflow_NonCriticalDropped(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logs.Buffer = 1

	b := NewBus(cfg, nil, nil)
	defer b.Close()

	ch := b.Subscribe(t.Context())

	b.Publish(Message{Type: EventPhaseChanged})

	b.Publish(Message{Type: EventServiceStarting, Critical: true})

	b.Publish(Message{Type: EventResourceSample, Critical: false})
	b.Publish(Message{Type: EventResourceSample, Critical: false})

	received := make([]MessageType, 0, 2)
	timeout := time.After(time.Second)

	for range 2 {
		select {
		case msg := <-ch:
			received = append(received, msg.Type)
		case <-timeout:
			t.Fatalf("timed out after %d/2 messages", len(received))
		}
	}

	assert.Equal(t, EventPhaseChanged, received[0])
	assert.Equal(t, EventServiceStarting, received[1])

	select {
	case <-ch:
		t.Fatal("non-critical traffic must not be retained")
	case <-time.After(50 * time.Millisecond):
	}
}
