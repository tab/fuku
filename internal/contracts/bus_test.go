package contracts

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// queue is a Subscription over a plain channel
type queue chan Message

func (q queue) Messages() <-chan Message {
	return q
}

func Test_Run_HandlesInOrder(t *testing.T) {
	messages := make(queue, 3)
	messages <- Message{Type: EventPhaseChanged}

	messages <- Message{Type: EventTierStarting}

	messages <- Message{Type: EventServiceReady}

	handled := make(chan MessageType, 3)

	loop := Run(t.Context(), messages, func(msg Message) {
		handled <- msg.Type
	})

	err := loop.Drain(t.Context())

	require.NoError(t, err)
	assert.Equal(t, EventPhaseChanged, <-handled)
	assert.Equal(t, EventTierStarting, <-handled)
	assert.Equal(t, EventServiceReady, <-handled)
}

func Test_Loop_Done(t *testing.T) {
	var handled atomic.Int32

	handle := func(Message) {
		handled.Add(1)
	}

	tests := []struct {
		name   string
		before func(t *testing.T) *Loop
	}{
		{
			name: "closes when the subscription closes",
			before: func(t *testing.T) *Loop {
				messages := make(queue)

				loop := Run(t.Context(), messages, handle)

				close(messages)

				return loop
			},
		},
		{
			name: "closes when the context is cancelled",
			before: func(t *testing.T) *Loop {
				ctx, cancel := context.WithCancel(t.Context())

				loop := Run(ctx, make(queue), handle)

				cancel()

				return loop
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loop := tt.before(t)

			<-loop.Done()

			assert.Equal(t, int32(0), handled.Load())
		})
	}
}

func Test_Loop_Drain_WaitsForInFlightHandler(t *testing.T) {
	messages := make(queue, 3)
	messages <- Message{Type: EventPhaseChanged}

	messages <- Message{Type: EventTierStarting}

	messages <- Message{Type: EventServiceReady}

	entered := make(chan struct{})
	release := make(chan struct{})
	drained := make(chan error, 1)

	var handled atomic.Int32

	loop := Run(t.Context(), messages, func(msg Message) {
		if msg.Type == EventPhaseChanged {
			close(entered)
			<-release
		}

		handled.Add(1)
	})

	<-entered

	go func() {
		drained <- loop.Drain(t.Context())
	}()

	close(release)

	err := <-drained

	require.NoError(t, err)
	assert.Equal(t, int32(3), handled.Load(), "Drain returned before the queued messages were handled")
}

func Test_Loop_Drain_ReturnsWhenLoopStopped(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	messages := make(queue, 1)
	messages <- Message{Type: EventPhaseChanged}

	loop := Run(ctx, messages, func(Message) {
		cancel()
	})

	<-loop.done

	err := loop.Drain(t.Context())

	require.NoError(t, err)
}

func Test_Loop_Drain_ContextCancelled(t *testing.T) {
	messages := make(queue, 1)
	messages <- Message{Type: EventPhaseChanged}

	entered := make(chan struct{})

	release := make(chan struct{})
	defer close(release)

	loop := Run(t.Context(), messages, func(Message) {
		close(entered)
		<-release
	})

	<-entered

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := loop.Drain(ctx)

	require.ErrorIs(t, err, context.Canceled)
}
