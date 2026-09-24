package bus

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Bus_Subscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporter := NewMockFailureReporter(ctrl)

	log := slog.New(slog.DiscardHandler)

	tests := []struct {
		name       string
		before     func() *Bus
		opts       contracts.SubscribeOptions
		subscribed bool
		expected   error
	}{
		{
			name: "named subscription",
			before: func() *Bus {
				return NewBus(Options{QueueDepth: 1}, mockReporter, log)
			},
			opts:       contracts.SubscribeOptions{Name: "store", Required: true},
			subscribed: true,
		},
		{
			name: "unnamed subscription",
			before: func() *Bus {
				return NewBus(Options{QueueDepth: 1}, mockReporter, log)
			},
			opts:     contracts.SubscribeOptions{},
			expected: ErrUnnamedSubscription,
		},
		{
			name: "after close",
			before: func() *Bus {
				b := NewBus(Options{QueueDepth: 1}, mockReporter, log)
				b.Close()

				return b
			},
			opts:     contracts.SubscribeOptions{Name: "store"},
			expected: contracts.ErrBusClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.before()
			defer b.Close()

			sub, err := b.Subscribe(t.Context(), tt.opts)

			require.ErrorIs(t, err, tt.expected)
			assert.Equal(t, tt.subscribed, sub != nil)
		})
	}
}

func Test_Bus_Publish_FIFO(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporter := NewMockFailureReporter(ctrl)

	log := slog.New(slog.DiscardHandler)

	b := NewBus(Options{QueueDepth: 8}, mockReporter, log)
	defer b.Close()

	sub, err := b.Subscribe(t.Context(), contracts.SubscribeOptions{Name: "store", Required: true})
	require.NoError(t, err)

	for pid := range 5 {
		err := b.Publish(contracts.Message{
			Type: contracts.EventServiceStarting,
			Data: contracts.ServiceStarting{ServiceEvent: contracts.ServiceEvent{Service: model.Service{ID: "id-api", Name: "api"}, Tier: "default"}, PID: pid},
		})

		require.NoError(t, err)
	}

	for expected := range 5 {
		msg := <-sub.Messages()

		assert.False(t, msg.Timestamp.IsZero())
		assert.Equal(t, expected, msg.Data.(contracts.ServiceStarting).PID)
	}
}

func Test_Bus_Publish_Filter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporter := NewMockFailureReporter(ctrl)

	log := slog.New(slog.DiscardHandler)

	b := NewBus(Options{QueueDepth: 8}, mockReporter, log)
	defer b.Close()

	commands, err := b.Subscribe(t.Context(), contracts.SubscribeOptions{Name: "runner", Required: true, Types: []contracts.MessageType{contracts.CommandStopAll}})
	require.NoError(t, err)

	everything, err := b.Subscribe(t.Context(), contracts.SubscribeOptions{Name: "eventlog"})
	require.NoError(t, err)

	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseRunning}}))
	require.NoError(t, b.Publish(contracts.Message{Type: contracts.CommandStopAll}))

	command := <-commands.Messages()
	first := <-everything.Messages()
	second := <-everything.Messages()

	assert.Equal(t, contracts.CommandStopAll, command.Type)
	assert.Equal(t, contracts.EventPhaseChanged, first.Type)
	assert.Equal(t, contracts.CommandStopAll, second.Type)
	assert.Empty(t, commands.Messages())
}

func Test_Bus_Publish_RequiredQueueFull(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporter := NewMockFailureReporter(ctrl)
	mockLog := NewMockLogger(ctrl)

	b := NewBus(Options{QueueDepth: 1}, mockReporter, mockLog)
	defer b.Close()

	store, err := b.Subscribe(t.Context(), contracts.SubscribeOptions{Name: "store", Required: true, Types: []contracts.MessageType{contracts.EventPhaseChanged}})
	require.NoError(t, err)

	runner, err := b.Subscribe(t.Context(), contracts.SubscribeOptions{Name: "runner", Required: true, Types: []contracts.MessageType{contracts.CommandStopAll}})
	require.NoError(t, err)

	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseStartup}}))

	overloaded := func(err error) {
		assert.ErrorContains(t, err, "bus overloaded: required subscription 'store' cannot accept phase_changed")
	}

	tests := []struct {
		name          string
		before        func()
		msg           contracts.Message
		expectedQueue int
		expected      error
	}{
		{
			name: "critical message is rejected without delivery",
			before: func() {
				mockLog.EXPECT().Error("Bus rejected a critical message", "error", gomock.Any(), "type", "phase_changed", "subscription", "store")
				mockReporter.EXPECT().Fail(gomock.Any()).Do(overloaded)
			},
			msg:           contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseRunning}},
			expectedQueue: 0,
			expected:      contracts.ErrBusOverloaded,
		},
		{
			name:          "non-critical message is dropped",
			before:        func() {},
			msg:           contracts.Message{Type: contracts.EventResourceSampled, Data: contracts.ResourceSampled{CPU: 1}},
			expectedQueue: 0,
		},
		{
			name:          "critical message the full queue does not match is delivered",
			before:        func() {},
			msg:           contracts.Message{Type: contracts.CommandStopAll},
			expectedQueue: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := b.Publish(tt.msg)

			require.ErrorIs(t, err, tt.expected)
			assert.Len(t, runner.Messages(), tt.expectedQueue)
			assert.Len(t, store.Messages(), 1)
		})
	}
}

func Test_Bus_Publish_RejectedMessageIsNeverDelivered(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporter := NewMockFailureReporter(ctrl)
	mockLog := NewMockLogger(ctrl)

	b := NewBus(Options{QueueDepth: 1}, mockReporter, mockLog)
	defer b.Close()

	store, err := b.Subscribe(t.Context(), contracts.SubscribeOptions{Name: "store", Required: true})
	require.NoError(t, err)

	mockLog.EXPECT().Error("Bus rejected a critical message", "error", gomock.Any(), "type", "phase_changed", "subscription", "store")
	mockReporter.EXPECT().Fail(gomock.Any())

	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseStartup}}))

	rejected := b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseRunning}})
	first := <-store.Messages()

	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseRunning}}))

	second := <-store.Messages()

	require.ErrorIs(t, rejected, contracts.ErrBusOverloaded)
	assert.Equal(t, model.PhaseStartup, first.Data.(contracts.PhaseChanged).Phase)
	assert.Equal(t, model.PhaseRunning, second.Data.(contracts.PhaseChanged).Phase)
}

func Test_Bus_Publish_OptionalDrops(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporter := NewMockFailureReporter(ctrl)
	mockLog := NewMockLogger(ctrl)

	b := NewBus(Options{QueueDepth: 1}, mockReporter, mockLog)

	tui, err := b.Subscribe(t.Context(), contracts.SubscribeOptions{Name: "tui"})
	require.NoError(t, err)

	mockLog.EXPECT().Warn("Subscription 'tui' dropped 2 phase_changed messages", "subscription", "tui", "type", "phase_changed", "dropped", uint64(2))
	mockLog.EXPECT().Warn("Subscription 'tui' dropped 1 resource_sample messages", "subscription", "tui", "type", "resource_sample", "dropped", uint64(1))

	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseStartup}}))
	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseRunning}}))
	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseStopping}}))
	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventResourceSampled, Data: contracts.ResourceSampled{CPU: 1}}))

	b.Close()

	delivered := <-tui.Messages()
	_, open := <-tui.Messages()

	assert.Equal(t, model.PhaseStartup, delivered.Data.(contracts.PhaseChanged).Phase)
	assert.False(t, open)
}

func Test_Bus_Publish_Closed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporter := NewMockFailureReporter(ctrl)

	log := slog.New(slog.DiscardHandler)
	b := NewBus(Options{QueueDepth: 1}, mockReporter, log)

	b.Close()
	b.Close()

	tests := []struct {
		name     string
		msg      contracts.Message
		expected error
	}{
		{
			name:     "critical",
			msg:      contracts.Message{Type: contracts.EventPhaseChanged},
			expected: contracts.ErrBusClosed,
		},
		{
			name: "non-critical",
			msg:  contracts.Message{Type: contracts.EventResourceSampled},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := b.Publish(tt.msg)

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Bus_Close_DeliversQueuedMessages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporter := NewMockFailureReporter(ctrl)

	log := slog.New(slog.DiscardHandler)
	b := NewBus(Options{QueueDepth: 4}, mockReporter, log)

	store, err := b.Subscribe(t.Context(), contracts.SubscribeOptions{Name: "store", Required: true})
	require.NoError(t, err)

	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseStopping}}))
	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventServiceStopped, Data: contracts.ServiceStopped{ServiceEvent: contracts.ServiceEvent{Service: model.Service{ID: "id-api", Name: "api"}}}}))
	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseStopped}}))

	b.Close()

	var received []contracts.MessageType

	for msg := range store.Messages() {
		received = append(received, msg.Type)
	}

	assert.Equal(t, []contracts.MessageType{contracts.EventPhaseChanged, contracts.EventServiceStopped, contracts.EventPhaseChanged}, received)
}

func Test_Bus_Unsubscribe_OnContextCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporter := NewMockFailureReporter(ctrl)

	log := slog.New(slog.DiscardHandler)

	b := NewBus(Options{QueueDepth: 1}, mockReporter, log)
	defer b.Close()

	ctx, cancel := context.WithCancel(t.Context())

	store, err := b.Subscribe(ctx, contracts.SubscribeOptions{Name: "store", Required: true})
	require.NoError(t, err)

	cancel()

	_, open := <-store.Messages()

	first := b.Publish(contracts.Message{Type: contracts.EventPhaseChanged})
	second := b.Publish(contracts.Message{Type: contracts.EventPhaseChanged})

	assert.False(t, open)
	require.NoError(t, first, "a removed required subscription no longer reserves slots")
	require.NoError(t, second)
}

func Test_Bus_Publish_ReentrantFromRequiredHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporter := NewMockFailureReporter(ctrl)

	log := slog.New(slog.DiscardHandler)

	b := NewBus(Options{QueueDepth: 4}, mockReporter, log)
	defer b.Close()

	runner, err := b.Subscribe(t.Context(), contracts.SubscribeOptions{Name: "runner", Required: true, Types: []contracts.MessageType{contracts.CommandStopAll}})
	require.NoError(t, err)

	store, err := b.Subscribe(t.Context(), contracts.SubscribeOptions{Name: "store", Required: true})
	require.NoError(t, err)

	published := make(chan error, 1)
	react := func(contracts.Message) {
		published <- b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseStopping}})
	}

	loop := contracts.Run(t.Context(), runner, react)

	require.NoError(t, b.Publish(contracts.Message{Type: contracts.CommandStopAll}))

	command := <-store.Messages()
	reaction := <-store.Messages()

	require.NoError(t, <-published)
	require.NoError(t, loop.Drain(t.Context()))
	assert.Equal(t, contracts.CommandStopAll, command.Type)
	assert.Equal(t, contracts.EventPhaseChanged, reaction.Type)
}

func Test_Bus_Publish_FullOwnInbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporter := NewMockFailureReporter(ctrl)
	mockLog := NewMockLogger(ctrl)

	b := NewBus(Options{QueueDepth: 1}, mockReporter, mockLog)
	defer b.Close()

	store, err := b.Subscribe(t.Context(), contracts.SubscribeOptions{Name: "store", Required: true})
	require.NoError(t, err)

	mockLog.EXPECT().Error("Bus rejected a critical message", "error", gomock.Any(), "type", "service_stopped", "subscription", "store")
	mockReporter.EXPECT().Fail(gomock.Any())

	entered := make(chan struct{})
	filled := make(chan struct{})
	published := make(chan error, 1)

	blockOnPhase := func(msg contracts.Message) {
		if msg.Type != contracts.EventPhaseChanged {
			return
		}

		close(entered)
		<-filled

		published <- b.Publish(contracts.Message{Type: contracts.EventServiceStopped, Data: contracts.ServiceStopped{}})
	}

	contracts.Run(t.Context(), store, blockOnPhase)

	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseRunning}}))

	<-entered

	require.NoError(t, b.Publish(contracts.Message{Type: contracts.EventTierStarting, Data: contracts.TierStarting{Name: "default"}}))

	close(filled)

	err = <-published

	require.ErrorIs(t, err, contracts.ErrBusOverloaded, "a handler's own full inbox rejects at once instead of waiting on itself")
}

func Test_Bus_Publish_DuringUnsubscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporter := NewMockFailureReporter(ctrl)

	log := slog.New(slog.DiscardHandler)

	b := NewBus(Options{QueueDepth: 1024}, mockReporter, log)
	defer b.Close()

	ctx, cancel := context.WithCancel(t.Context())

	store, err := b.Subscribe(ctx, contracts.SubscribeOptions{Name: "store", Required: true})
	require.NoError(t, err)

	var wg sync.WaitGroup

	errs := make(chan error, 64)

	publish := func() {
		errs <- b.Publish(contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseRunning}})
	}

	for range 64 {
		wg.Go(publish)
	}

	cancel()
	wg.Wait()
	close(errs)

	for msg := range store.Messages() {
		assert.Equal(t, contracts.EventPhaseChanged, msg.Type)
	}

	for err := range errs {
		require.NoError(t, err)
	}
}
