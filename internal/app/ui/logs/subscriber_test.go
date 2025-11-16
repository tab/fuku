package logs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/runtime"
	"fuku/internal/app/ui"
)

func Test_NewSubscriber(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEventBus := runtime.NewMockEventBus(ctrl)
	mockLogView := ui.NewMockLogView(ctrl)

	subscriber := NewSubscriber(mockEventBus, mockLogView)
	assert.NotNil(t, subscriber)
	assert.Equal(t, mockEventBus, subscriber.eventBus)
	assert.Equal(t, mockLogView, subscriber.logView)
}

func Test_Subscriber_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEventBus := runtime.NewMockEventBus(ctrl)
	mockLogView := ui.NewMockLogView(ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventChan := make(chan runtime.Event, 10)
	mockEventBus.EXPECT().Subscribe(ctx).Return(eventChan)

	subscriber := NewSubscriber(mockEventBus, mockLogView)
	subscriber.Start(ctx)

	time.Sleep(10 * time.Millisecond)
}

func Test_Subscriber_ProcessEvents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogView := ui.NewMockLogView(ctrl)

	tests := []struct {
		name      string
		event     runtime.Event
		expectLog bool
	}{
		{
			name: "EventLogLine forwards to LogView",
			event: runtime.Event{
				Type:      runtime.EventLogLine,
				Timestamp: time.Now(),
				Data:      runtime.LogLineData{Service: "api", Tier: "tier1", Stream: "STDOUT", Message: "test"},
			},
			expectLog: true,
		},
		{
			name: "EventServiceReady ignored",
			event: runtime.Event{
				Type:      runtime.EventServiceReady,
				Timestamp: time.Now(),
				Data:      runtime.ServiceReadyData{Service: "api", Tier: "tier1"},
			},
			expectLog: false,
		},
		{
			name: "Invalid data ignored",
			event: runtime.Event{
				Type:      runtime.EventLogLine,
				Timestamp: time.Now(),
				Data:      "invalid",
			},
			expectLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventChan := make(chan runtime.Event, 1)
			eventChan <- tt.event

			close(eventChan)

			if tt.expectLog {
				mockLogView.EXPECT().HandleLog(gomock.Any()).Do(func(entry ui.LogEntry) {
					assert.Equal(t, "api", entry.Service)
					assert.Equal(t, "tier1", entry.Tier)
					assert.Equal(t, "STDOUT", entry.Stream)
					assert.Equal(t, "test", entry.Message)
				})
			}

			subscriber := &Subscriber{logView: mockLogView}
			subscriber.processEvents(eventChan)
		})
	}
}
