package logs

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/runtime"
)

func Test_NewSubscriber(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEventBus := runtime.NewMockEventBus(ctrl)
	sendFunc := NewSender()

	subscriber := NewSubscriber(mockEventBus, sendFunc)
	assert.NotNil(t, subscriber)
	assert.Equal(t, mockEventBus, subscriber.eventBus)
	assert.Equal(t, sendFunc, subscriber.sender)
}

func Test_Subscriber_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEventBus := runtime.NewMockEventBus(ctrl)
	sendFunc := NewSender()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventChan := make(chan runtime.Event, 10)
	mockEventBus.EXPECT().Subscribe(ctx).Return(eventChan)

	subscriber := NewSubscriber(mockEventBus, sendFunc)
	subscriber.Start(ctx)
}

func Test_Subscriber_StartCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEventBus := runtime.NewMockEventBus(ctrl)
	sendFunc := NewSender()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventChan := make(chan runtime.Event, 10)
	mockEventBus.EXPECT().Subscribe(ctx).Return(eventChan)

	subscriber := NewSubscriber(mockEventBus, sendFunc)
	cmd := subscriber.StartCmd(ctx)

	assert.NotNil(t, cmd)

	msg := cmd()
	assert.Nil(t, msg)
}

func Test_Subscriber_ProcessEvents(t *testing.T) {
	tests := []struct {
		name      string
		event     runtime.Event
		expectMsg bool
	}{
		{
			name: "EventLogLine sends LogMsg",
			event: runtime.Event{
				Type:      runtime.EventLogLine,
				Timestamp: time.Now(),
				Data:      runtime.LogLineData{Service: "api", Tier: "tier1", Stream: "STDOUT", Message: "test"},
			},
			expectMsg: true,
		},
		{
			name: "EventServiceReady ignored",
			event: runtime.Event{
				Type:      runtime.EventServiceReady,
				Timestamp: time.Now(),
				Data:      runtime.ServiceReadyData{Service: "api", Tier: "tier1"},
			},
			expectMsg: false,
		},
		{
			name: "Invalid data ignored",
			event: runtime.Event{
				Type:      runtime.EventLogLine,
				Timestamp: time.Now(),
				Data:      "invalid",
			},
			expectMsg: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventChan := make(chan runtime.Event, 1)
			eventChan <- tt.event

			close(eventChan)

			var receivedMsg tea.Msg

			sendFunc := NewSender()
			sendFunc.Set(func(msg tea.Msg) {
				receivedMsg = msg
			})

			subscriber := &Subscriber{sender: sendFunc}
			subscriber.processEvents(eventChan)

			if tt.expectMsg {
				assert.NotNil(t, receivedMsg)
				logMsg, ok := receivedMsg.(LogMsg)
				assert.True(t, ok)
				assert.Equal(t, "api", logMsg.Service)
				assert.Equal(t, "tier1", logMsg.Tier)
				assert.Equal(t, "STDOUT", logMsg.Stream)
				assert.Equal(t, "test", logMsg.Message)
			} else {
				assert.Nil(t, receivedMsg)
			}
		})
	}
}
