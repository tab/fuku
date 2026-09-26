package contracts

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"

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

func Test_Loop_run_ClosesDone(t *testing.T) {
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

			<-loop.done

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

func Test_Loop_Drain_ContextCancelledBehindQueuedMessages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		first := make(chan struct{})

		last := make(chan struct{})
		defer close(last)

		// sync: behind 64 queued messages the loop's select skips the drain request only with odds of 2^-65
		queued := 64

		messages := make(queue, queued+2)
		messages <- Message{Type: EventPhaseChanged}

		for range queued {
			messages <- Message{Type: EventServiceReady}
		}

		messages <- Message{Type: EventTierStarting}

		ctx, cancel := context.WithCancel(t.Context())
		drained := make(chan error, 1)
		handle := func(msg Message) {
			if msg.Type == EventPhaseChanged {
				<-first
			}

			if msg.Type == EventTierStarting {
				<-last
			}
		}

		loop := Run(t.Context(), messages, handle)
		drain := func() {
			drained <- loop.Drain(ctx)
		}

		synctest.Wait()

		go drain()

		synctest.Wait()
		close(first)
		synctest.Wait()
		cancel()

		err := <-drained

		require.ErrorIs(t, err, context.Canceled)
	})
}

func Test_Loop_Drain_ReturnsWhenLoopStoppedBehindQueuedMessages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())

		first := make(chan struct{})
		last := make(chan struct{})

		// sync: behind 64 queued messages the loop's select skips the drain request or the cancel only with odds of 2^-64
		queued := 64

		messages := make(queue, 2*queued+2)
		messages <- Message{Type: EventPhaseChanged}

		for range queued {
			messages <- Message{Type: EventServiceReady}
		}

		messages <- Message{Type: EventTierStarting}

		for range queued {
			messages <- Message{Type: EventServiceReady}
		}

		drained := make(chan error, 1)
		handle := func(msg Message) {
			if msg.Type == EventPhaseChanged {
				<-first
			}

			if msg.Type == EventTierStarting {
				<-last
			}
		}

		loop := Run(ctx, messages, handle)
		drain := func() {
			drained <- loop.Drain(t.Context())
		}

		synctest.Wait()

		go drain()

		synctest.Wait()
		close(first)
		synctest.Wait()
		cancel()
		close(last)

		err := <-drained

		require.NoError(t, err)
		assert.NotEmpty(t, messages, "the loop emptied the queue before it stopped")
	})
}
