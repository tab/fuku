package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_CommandBus_Publish_And_Subscribe(t *testing.T) {
	cb := NewCommandBus(10)
	defer cb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := cb.Subscribe(ctx)

	cmd := Command{
		Type: CommandStopService,
		Data: StopServiceData{Service: "test-service"},
	}

	cb.Publish(cmd)

	select {
	case received := <-sub:
		assert.Equal(t, CommandStopService, received.Type)
		data, ok := received.Data.(StopServiceData)
		assert.True(t, ok)
		assert.Equal(t, "test-service", data.Service)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for command")
	}
}

func Test_CommandBus_Multiple_Subscribers(t *testing.T) {
	cb := NewCommandBus(10)
	defer cb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub1 := cb.Subscribe(ctx)
	sub2 := cb.Subscribe(ctx)

	cmd := Command{Type: CommandStopAll}

	cb.Publish(cmd)

	select {
	case received := <-sub1:
		assert.Equal(t, CommandStopAll, received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for command on sub1")
	}

	select {
	case received := <-sub2:
		assert.Equal(t, CommandStopAll, received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for command on sub2")
	}
}

func Test_CommandBus_Unsubscribe_On_Context_Cancel(t *testing.T) {
	cb := NewCommandBus(10)
	defer cb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	sub := cb.Subscribe(ctx)

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

func Test_CommandBus_No_Commands_After_Context_Cancel(t *testing.T) {
	cb := NewCommandBus(10)
	defer cb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	sub := cb.Subscribe(ctx)

	cmd1 := Command{Type: CommandStopService, Data: StopServiceData{Service: "test1"}}
	cb.Publish(cmd1)

	select {
	case received := <-sub:
		assert.Equal(t, CommandStopService, received.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for command before cancel")
	}

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

func Test_CommandBus_Close_Closes_All_Subscribers(t *testing.T) {
	cb := NewCommandBus(10)

	ctx := context.Background()
	sub1 := cb.Subscribe(ctx)
	sub2 := cb.Subscribe(ctx)

	cb.Close()

	_, ok1 := <-sub1
	_, ok2 := <-sub2

	assert.False(t, ok1, "sub1 should be closed")
	assert.False(t, ok2, "sub2 should be closed")
}

func Test_CommandBus_Publish_After_Close_Does_Not_Panic(t *testing.T) {
	cb := NewCommandBus(10)
	cb.Close()

	assert.NotPanics(t, func() {
		cb.Publish(Command{Type: CommandStopAll})
	})
}

func Test_CommandBus_Buffer_Full_Does_Not_Block(t *testing.T) {
	cb := NewCommandBus(1)
	defer cb.Close()

	ctx := context.Background()
	cb.Subscribe(ctx)

	for i := 0; i < 10; i++ {
		cb.Publish(Command{Type: CommandStopService})
	}
}

func Test_NoOpCommandBus_Returns_Closed_Channel(t *testing.T) {
	ncb := NewNoOpCommandBus()
	defer ncb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	sub := ncb.Subscribe(ctx)

	cancel()

	assert.Eventually(t, func() bool {
		select {
		case _, ok := <-sub:
			return !ok
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond, "no-op command bus should return closed channel after context cancel")
}

func Test_NoOpCommandBus_Publish_Does_Not_Panic(t *testing.T) {
	ncb := NewNoOpCommandBus()
	defer ncb.Close()

	assert.NotPanics(t, func() {
		ncb.Publish(Command{Type: CommandStopService})
	})
}
