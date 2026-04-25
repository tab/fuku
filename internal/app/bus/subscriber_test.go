package bus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_NewSubscriber(t *testing.T) {
	sub := newSubscriber(10)
	defer sub.close()

	assert.NotNil(t, sub.ch)
	assert.Equal(t, 10, cap(sub.ch))
}

func Test_Subscriber_Send(t *testing.T) {
	sub := newSubscriber(5)
	defer sub.close()

	sub.send(Message{Type: EventPhaseChanged})

	select {
	case msg := <-sub.ch:
		assert.Equal(t, EventPhaseChanged, msg.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("message not received")
	}
}

func Test_Subscriber_Send_DropsNonCriticalWhenFull(t *testing.T) {
	sub := newSubscriber(1)
	defer sub.close()

	sub.ch <- Message{Type: EventPhaseChanged}

	sub.send(Message{Type: EventResourceSample, Critical: false})

	sub.mu.Lock()
	overflowLen := len(sub.overflow)
	sub.mu.Unlock()

	assert.Equal(t, 0, overflowLen)
}

func Test_Subscriber_Send_QueuesCriticalWhenFull(t *testing.T) {
	sub := newSubscriber(1)
	defer sub.close()

	sub.ch <- Message{Type: EventPhaseChanged}

	sub.send(Message{Type: EventServiceReady, Critical: true})

	sub.mu.Lock()
	overflowLen := len(sub.overflow)
	sub.mu.Unlock()

	assert.Equal(t, 1, overflowLen)
}

func Test_Subscriber_Send_SkipsWhenClosed(t *testing.T) {
	sub := newSubscriber(5)
	sub.close()

	sub.send(Message{Type: EventPhaseChanged})

	sub.mu.Lock()
	overflowLen := len(sub.overflow)
	sub.mu.Unlock()

	assert.Equal(t, 0, overflowLen)
}

func Test_Subscriber_Send_CriticalSkipsWhenClosed(t *testing.T) {
	sub := newSubscriber(1)

	sub.ch <- Message{Type: EventPhaseChanged}

	sub.close()

	sub.send(Message{Type: EventServiceReady, Critical: true})

	sub.mu.Lock()
	overflowLen := len(sub.overflow)
	sub.mu.Unlock()

	assert.Equal(t, 0, overflowLen)
}

func Test_Subscriber_OverflowFIFO(t *testing.T) {
	sub := newSubscriber(1)
	defer sub.close()

	sub.ch <- Message{Type: EventPhaseChanged}

	sub.send(Message{Type: EventServiceStarting, Critical: true, Seq: 1})
	sub.send(Message{Type: EventServiceReady, Critical: true, Seq: 2})

	sub.mu.Lock()
	assert.Len(t, sub.overflow, 2)
	assert.Equal(t, EventServiceStarting, sub.overflow[0].Type)
	assert.Equal(t, EventServiceReady, sub.overflow[1].Type)
	sub.mu.Unlock()
}

func Test_Subscriber_PumpDrainsOverflow(t *testing.T) {
	sub := newSubscriber(1)
	defer sub.close()

	sub.ch <- Message{Type: EventPhaseChanged}

	sub.send(Message{Type: EventServiceStarting, Critical: true})
	sub.send(Message{Type: EventServiceReady, Critical: true})

	received := make([]MessageType, 0, 3)
	timeout := time.After(time.Second)

	for range 3 {
		select {
		case msg := <-sub.ch:
			received = append(received, msg.Type)
		case <-timeout:
			t.Fatalf("timed out after %d/3 messages", len(received))
		}
	}

	assert.Equal(t, []MessageType{
		EventPhaseChanged,
		EventServiceStarting,
		EventServiceReady,
	}, received)
}

func Test_Subscriber_Close(t *testing.T) {
	sub := newSubscriber(5)

	sub.send(Message{Type: EventPhaseChanged})

	sub.close()

	_, ok := <-sub.ch
	if ok {
		_, ok = <-sub.ch
	}

	assert.False(t, ok, "channel should be closed")
}

func Test_Subscriber_Close_Idempotent(t *testing.T) {
	sub := newSubscriber(5)

	sub.close()
	sub.close()
}

func Test_Subscriber_Close_WithPendingOverflow(t *testing.T) {
	sub := newSubscriber(1)

	sub.ch <- Message{Type: EventPhaseChanged}

	sub.send(Message{Type: EventServiceStarting, Critical: true})
	sub.send(Message{Type: EventServiceReady, Critical: true})

	sub.close()

	drained := 0

	for range sub.ch {
		drained++
	}

	assert.GreaterOrEqual(t, drained, 1)
}

func Test_Subscriber_Send_DropsNonCriticalWhileOverflowDrains(t *testing.T) {
	sub := newSubscriber(1)
	defer sub.close()

	sub.ch <- Message{Type: EventPhaseChanged}

	sub.send(Message{Type: EventServiceStarting, Critical: true})

	sub.send(Message{Type: EventResourceSample, Critical: false})

	sub.mu.Lock()
	assert.Len(t, sub.overflow, 1)
	assert.Equal(t, EventServiceStarting, sub.overflow[0].Type)
	sub.mu.Unlock()
}
