package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_EventBus_Publish_And_Subscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eb := NewEventBus(10)
	defer eb.Close()

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eb := NewEventBus(10)
	defer eb.Close()

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

	assert.Eventually(t, func() bool {
		select {
		case _, ok := <-sub:
			return !ok
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond, "channel should be closed after context cancellation")
}

func Test_EventBus_Close_Closes_All_Subscribers(t *testing.T) {
	ctx := context.Background()
	eb := NewEventBus(10)
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
	ctx := context.Background()
	eb := NewEventBus(1)

	defer eb.Close()

	eb.Subscribe(ctx)

	for i := 0; i < 10; i++ {
		eb.Publish(Event{Type: EventServiceStarting})
	}

	// Test passes if no deadlock occurs
}

func Test_NoOpEventBus_Returns_Closed_Channel(t *testing.T) {
	ctx := context.Background()

	neb := NewNoOpEventBus()

	defer neb.Close()

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
	ctx := context.Background()
	eb := NewEventBus(2)

	defer eb.Close()

	sub := eb.Subscribe(ctx)
	receivedEvents := make(chan Event, 20)

	go func() {
		for event := range sub {
			receivedEvents <- event
		}
	}()

	for i := 0; i < 10; i++ {
		eb.Publish(Event{Type: EventTierReady, Critical: false})
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

func Test_EventBus_Close_Twice_Does_Not_Panic(t *testing.T) {
	eb := NewEventBus(10)

	assert.NotPanics(t, func() {
		eb.Close()
		eb.Close()
	})
}

func Test_EventBus_AllEventTypes(t *testing.T) {
	eventTypes := []EventType{
		EventProfileResolved,
		EventPhaseChanged,
		EventTierStarting,
		EventTierReady,
		EventTierFailed,
		EventServiceStarting,
		EventServiceReady,
		EventServiceFailed,
		EventServiceStopped,
		EventRetryScheduled,
		EventSignalCaught,
	}

	for _, et := range eventTypes {
		t.Run(string(et), func(t *testing.T) {
			eb := NewEventBus(10)
			defer eb.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sub := eb.Subscribe(ctx)

			eb.Publish(Event{Type: et})

			select {
			case received := <-sub:
				assert.Equal(t, et, received.Type)
			case <-time.After(100 * time.Millisecond):
				t.Fatalf("timeout waiting for event %s", et)
			}
		})
	}
}

func Test_EventBus_AllPhases(t *testing.T) {
	phases := []Phase{
		PhaseStartup,
		PhaseRunning,
		PhaseStopping,
		PhaseStopped,
	}

	for _, p := range phases {
		t.Run(string(p), func(t *testing.T) {
			data := PhaseChangedData{Phase: p}
			assert.Equal(t, p, data.Phase)
		})
	}
}

func Test_EventData_Types(t *testing.T) {
	t.Run("TierData", func(t *testing.T) {
		data := TierData{Name: "tier1", Services: []string{"svc1", "svc2"}}
		assert.Equal(t, "tier1", data.Name)
		assert.Len(t, data.Services, 2)
	})

	t.Run("ProfileResolvedData", func(t *testing.T) {
		data := ProfileResolvedData{
			Profile: "default",
			Tiers:   []TierData{{Name: "tier1"}},
		}
		assert.Equal(t, "default", data.Profile)
		assert.Len(t, data.Tiers, 1)
	})

	t.Run("TierStartingData", func(t *testing.T) {
		data := TierStartingData{Name: "tier1", Index: 0, Total: 3}
		assert.Equal(t, "tier1", data.Name)
		assert.Equal(t, 0, data.Index)
		assert.Equal(t, 3, data.Total)
	})

	t.Run("TierReadyData", func(t *testing.T) {
		data := TierReadyData{Name: "tier1"}
		assert.Equal(t, "tier1", data.Name)
	})

	t.Run("TierFailedData", func(t *testing.T) {
		data := TierFailedData{Name: "tier1", FailedServices: []string{"svc1"}, TotalServices: 2}
		assert.Equal(t, "tier1", data.Name)
		assert.Len(t, data.FailedServices, 1)
		assert.Equal(t, 2, data.TotalServices)
	})

	t.Run("ServiceStartingData", func(t *testing.T) {
		data := ServiceStartingData{Service: "svc1", Tier: "tier1", Attempt: 1, PID: 1234}
		assert.Equal(t, "svc1", data.Service)
		assert.Equal(t, "tier1", data.Tier)
		assert.Equal(t, 1, data.Attempt)
		assert.Equal(t, 1234, data.PID)
	})

	t.Run("ServiceReadyData", func(t *testing.T) {
		data := ServiceReadyData{Service: "svc1", Tier: "tier1", Duration: time.Second}
		assert.Equal(t, "svc1", data.Service)
		assert.Equal(t, time.Second, data.Duration)
	})

	t.Run("ServiceFailedData", func(t *testing.T) {
		data := ServiceFailedData{Service: "svc1", Tier: "tier1", Error: assert.AnError}
		assert.Equal(t, "svc1", data.Service)
		assert.Error(t, data.Error)
	})

	t.Run("ServiceStoppedData", func(t *testing.T) {
		data := ServiceStoppedData{Service: "svc1", Tier: "tier1"}
		assert.Equal(t, "svc1", data.Service)
	})

	t.Run("RetryScheduledData", func(t *testing.T) {
		data := RetryScheduledData{Service: "svc1", Attempt: 2, MaxAttempts: 3}
		assert.Equal(t, "svc1", data.Service)
		assert.Equal(t, 2, data.Attempt)
		assert.Equal(t, 3, data.MaxAttempts)
	})

	t.Run("SignalCaughtData", func(t *testing.T) {
		data := SignalCaughtData{Signal: "SIGINT"}
		assert.Equal(t, "SIGINT", data.Signal)
	})
}
