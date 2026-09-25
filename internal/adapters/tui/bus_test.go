package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
)

func Test_Bridge_Subscribe_HoldsMessagesUntilTheViewAttaches(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockView := NewMockView(ctrl)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	update := contracts.Message{Type: contracts.EventUpdateAvailable, Data: contracts.UpdateAvailable{Version: "v9.9.9"}}
	signal := contracts.Message{Type: contracts.EventSignalReceived, Data: contracts.SignalReceived{Name: "SIGINT"}}
	messages := make(queue, 2)
	delivered := make(chan struct{})
	countDelivered := func(tea.Msg) { delivered <- struct{}{} }

	mockSubscriber.EXPECT().Subscribe(ctx, contracts.SubscribeOptions{Name: "tui", Types: received}).Return(messages, nil)
	gomock.InOrder(
		mockView.EXPECT().Send(EventMsg(update)).Do(countDelivered),
		mockView.EXPECT().Send(EventMsg(signal)).Do(countDelivered),
	)

	messages <- update

	messages <- signal

	bridge := NewBridge(mockSubscriber)

	require.NoError(t, bridge.Subscribe(ctx))

	bridge.attach(mockView)

	<-delivered
	<-delivered
}

func Test_Bridge_Subscribe_ReturnsTheRejection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "tui", Types: received}).Return(nil, contracts.ErrBusClosed)

	bridge := NewBridge(mockSubscriber)

	err := bridge.Subscribe(t.Context())

	require.ErrorIs(t, err, contracts.ErrBusClosed)
}

func Test_Bridge_Forward_StopsWhenTheContextEndsBeforeAttach(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	ctx, cancel := context.WithCancel(t.Context())
	messages := make(queue)

	mockSubscriber.EXPECT().Subscribe(ctx, contracts.SubscribeOptions{Name: "tui", Types: received}).Return(messages, nil)

	bridge := NewBridge(mockSubscriber)

	require.NoError(t, bridge.Subscribe(ctx))

	messages <- contracts.Message{Type: contracts.EventSignalReceived}

	cancel()

	require.NoError(t, bridge.loop.Drain(t.Context()))

	assert.Nil(t, bridge.view)
}

func Test_Bridge_Drain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockView := NewMockView(ctrl)

	messages := make(queue, 1)
	delivered := make(chan struct{})
	countDelivered := func(tea.Msg) { close(delivered) }

	tests := []struct {
		name   string
		before func() *Bridge
	}{
		{
			name: "a bridge that never subscribed has nothing to drain",
			before: func() *Bridge {
				return NewBridge(mockSubscriber)
			},
		},
		{
			name: "a bridge without a view returns at once while its queue waits",
			before: func() *Bridge {
				bridge := NewBridge(mockSubscriber)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "tui", Types: received}).Return(make(queue, 1), nil)

				require.NoError(t, bridge.Subscribe(t.Context()))

				return bridge
			},
		},
		{
			name: "an attached bridge drains once the view received the queue",
			before: func() *Bridge {
				bridge := NewBridge(mockSubscriber)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "tui", Types: received}).Return(messages, nil)
				mockView.EXPECT().Send(gomock.Any()).Do(countDelivered)

				messages <- contracts.Message{Type: contracts.EventSignalReceived}

				require.NoError(t, bridge.Subscribe(t.Context()))

				bridge.attach(mockView)

				return bridge
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := tt.before()

			err := bridge.Drain(t.Context())

			require.NoError(t, err)
		})
	}

	<-delivered
}

func Test_Bridge_Forward_ReachesAnAttachedView(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockView := NewMockView(ctrl)

	update := contracts.Message{Type: contracts.EventUpdateAvailable, Data: contracts.UpdateAvailable{Version: "v9.9.9"}}
	messages := make(queue, 1)
	delivered := make(chan struct{})
	countDelivered := func(tea.Msg) { close(delivered) }

	mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "tui", Types: received}).Return(messages, nil)
	mockView.EXPECT().Send(EventMsg(update)).Do(countDelivered)

	bridge := NewBridge(mockSubscriber)

	require.NoError(t, bridge.Subscribe(t.Context()))

	bridge.attach(mockView)

	messages <- update

	<-delivered
}
