package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_EventBus_Publish_And_Subscribe(t *testing.T) {
	eb := NewEventBus(10)
	defer eb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := eb.Subscribe(ctx)

	event := Event{
		Type: EventServiceStarting,
		Data: ServiceStartingData{Service: "test-service", Tier: "default", Attempt: 1},
	}

	eb.Publish(event)

	select {
	case received := <-sub:
		assert.Equal(t, EventServiceStarting, received.Type)
		assert.NotZero(t, received.Timestamp)
		data, ok := received.Data.(ServiceStartingData)
		assert.True(t, ok)
		assert.Equal(t, "test-service", data.Service)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func Test_EventBus_Multiple_Subscribers(t *testing.T) {
	eb := NewEventBus(10)
	defer eb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub1 := eb.Subscribe(ctx)
	sub2 := eb.Subscribe(ctx)

	event := Event{Type: EventPhaseChanged, Data: PhaseChangedData{Phase: PhaseRunning}}

	eb.Publish(event)

	select {
	case received := <-sub1:
		assert.Equal(t, EventPhaseChanged, received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event on sub1")
	}

	select {
	case received := <-sub2:
		assert.Equal(t, EventPhaseChanged, received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event on sub2")
	}
}

func Test_EventBus_Unsubscribe_On_Context_Cancel(t *testing.T) {
	eb := NewEventBus(10)
	defer eb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	sub := eb.Subscribe(ctx)

	cancel()
	time.Sleep(10 * time.Millisecond)

	_, ok := <-sub
	assert.False(t, ok, "channel should be closed after context cancellation")
}

func Test_EventBus_Close_Closes_All_Subscribers(t *testing.T) {
	eb := NewEventBus(10)

	ctx := context.Background()
	sub1 := eb.Subscribe(ctx)
	sub2 := eb.Subscribe(ctx)

	eb.Close()

	_, ok1 := <-sub1
	_, ok2 := <-sub2

	assert.False(t, ok1, "sub1 should be closed")
	assert.False(t, ok2, "sub2 should be closed")
}

func Test_EventBus_Publish_After_Close_Does_Not_Panic(t *testing.T) {
	eb := NewEventBus(10)
	eb.Close()

	assert.NotPanics(t, func() {
		eb.Publish(Event{Type: EventServiceReady})
	})
}

func Test_EventBus_Buffer_Full_Does_Not_Block(t *testing.T) {
	eb := NewEventBus(1)
	defer eb.Close()

	ctx := context.Background()
	eb.Subscribe(ctx)

	for i := 0; i < 10; i++ {
		eb.Publish(Event{Type: EventServiceStarting})
	}
}

func Test_NoOpEventBus_Returns_Closed_Channel(t *testing.T) {
	neb := NewNoOpEventBus()
	defer neb.Close()

	ctx := context.Background()
	sub := neb.Subscribe(ctx)

	_, ok := <-sub
	assert.False(t, ok, "no-op event bus should return closed channel")
}

func Test_NoOpEventBus_Publish_Does_Not_Panic(t *testing.T) {
	neb := NewNoOpEventBus()
	defer neb.Close()

	assert.NotPanics(t, func() {
		neb.Publish(Event{Type: EventServiceReady})
	})
}

func Test_EventBus_Critical_Events_Always_Delivered(t *testing.T) {
	eb := NewEventBus(2)
	defer eb.Close()

	ctx := context.Background()
	sub := eb.Subscribe(ctx)

	receivedEvents := make(chan Event, 20)

	go func() {
		for event := range sub {
			receivedEvents <- event
		}
	}()

	for i := 0; i < 10; i++ {
		eb.Publish(Event{Type: EventLogLine, Critical: false})
	}

	criticalEvent := Event{Type: EventPhaseChanged, Data: PhaseChangedData{Phase: PhaseStopped}, Critical: true}
	eb.Publish(criticalEvent)

	var foundCritical bool

	timeout := time.After(500 * time.Millisecond)

	for {
		select {
		case received := <-receivedEvents:
			if received.Type == EventPhaseChanged && received.Critical {
				foundCritical = true
				goto done
			}
		case <-timeout:
			t.Fatal("timeout waiting for critical event")
		}
	}

done:
	assert.True(t, foundCritical, "critical event should have been delivered")
}
