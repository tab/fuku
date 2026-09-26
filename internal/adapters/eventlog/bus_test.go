package eventlog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

// queue is a Subscription over a plain channel
type queue chan contracts.Message

func (q queue) Messages() <-chan contracts.Message {
	return q
}

func Test_Recorder_Subscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockBroadcaster := NewMockBroadcaster(ctrl)
	mockLog := NewMockLogger(ctrl)

	formatter := NewFormatter()

	recorder := NewRecorder(mockSubscriber, mockBroadcaster, formatter, mockLog)

	tests := []struct {
		name     string
		before   func()
		expected error
	}{
		{
			name: "registers the optional eventlog subscription",
			before: func() {
				messages := make(queue)
				close(messages)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "eventlog"}).Return(messages, nil)
			},
		},
		{
			name: "returns the subscribe error",
			before: func() {
				mockSubscriber.EXPECT().Subscribe(gomock.Any(), contracts.SubscribeOptions{Name: "eventlog"}).Return(nil, contracts.ErrBusClosed)
			},
			expected: contracts.ErrBusClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := recorder.Subscribe(t.Context())

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Recorder_Drain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockBroadcaster := NewMockBroadcaster(ctrl)
	mockLog := NewMockLogger(ctrl)

	formatter := NewFormatter()

	recorder := NewRecorder(mockSubscriber, mockBroadcaster, formatter, mockLog)

	tests := []struct {
		name     string
		before   func() context.Context
		expected error
	}{
		{
			name: "returns once the queue is recorded",
			before: func() context.Context {
				messages := make(queue, 1)
				messages <- contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseStopped}}

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(messages, nil)
				mockLog.EXPECT().Debug("phase_changed duration=0s phase=stopped services=0")
				mockBroadcaster.EXPECT().Broadcast(buildinfo.AppName, "phase_changed duration=0s phase=stopped services=0")

				require.NoError(t, recorder.Subscribe(t.Context()))

				return t.Context()
			},
		},
		{
			name: "returns the context error while a handler is in flight",
			before: func() context.Context {
				messages := make(queue, 1)
				messages <- contracts.Message{Type: contracts.EventPhaseChanged, Data: contracts.PhaseChanged{Phase: model.PhaseStopped}}

				ctx, cancel := context.WithCancel(t.Context())
				release := make(chan struct{})

				t.Cleanup(func() { close(release) })

				blocked := func(string, string) {
					cancel()
					<-release
				}

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(messages, nil)
				mockLog.EXPECT().Debug("phase_changed duration=0s phase=stopped services=0")
				mockBroadcaster.EXPECT().Broadcast(buildinfo.AppName, "phase_changed duration=0s phase=stopped services=0").Do(blocked)

				require.NoError(t, recorder.Subscribe(t.Context()))

				return ctx
			},
			expected: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.before()

			err := recorder.Drain(ctx)

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Recorder_Record(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockBroadcaster := NewMockBroadcaster(ctrl)
	mockLog := NewMockLogger(ctrl)

	formatter := NewFormatter()

	recorder := NewRecorder(mockSubscriber, mockBroadcaster, formatter, mockLog)

	tests := []struct {
		name   string
		before func()
	}{
		{
			name: "logs every message at debug and broadcasts it in order",
			before: func() {
				messages := make(queue, 2)

				messages <- contracts.Message{
					Type: contracts.EventServiceStarting,
					Data: contracts.ServiceStarting{
						ServiceEvent: contracts.ServiceEvent{Service: model.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
						Attempt:      1,
						PID:          123,
					},
				}

				messages <- contracts.Message{
					Type: contracts.EventServiceReady,
					Data: contracts.ServiceReady{
						ServiceEvent: contracts.ServiceEvent{Service: model.Service{ID: "test-id-api", Name: "api"}, Tier: "platform"},
					},
				}

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(messages, nil)
				mockLog.EXPECT().Debug("service_starting attempt=1 id=test-id-api pid=123 service=api tier=platform")
				mockLog.EXPECT().Debug("service_ready id=test-id-api service=api tier=platform")
				gomock.InOrder(
					mockBroadcaster.EXPECT().Broadcast(buildinfo.AppName, "service_starting attempt=1 id=test-id-api pid=123 service=api tier=platform"),
					mockBroadcaster.EXPECT().Broadcast(buildinfo.AppName, "service_ready id=test-id-api service=api tier=platform"),
				)

				require.NoError(t, recorder.Subscribe(t.Context()))
			},
		},
		{
			name: "skips the read-model notifications",
			before: func() {
				messages := make(queue, 2)

				messages <- contracts.Message{
					Type: contracts.EventSnapshotChanged,
					Data: contracts.SnapshotChanged{},
				}

				messages <- contracts.Message{
					Type: contracts.EventServiceResourcesSampled,
					Data: contracts.ServiceResourcesSampled{Services: []contracts.ServiceResourceSample{{ID: "test-id-api", PID: 123}}},
				}

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(messages, nil)

				require.NoError(t, recorder.Subscribe(t.Context()))
			},
		},
		{
			name: "logs nothing for an empty subscription",
			before: func() {
				messages := make(queue)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(messages, nil)

				require.NoError(t, recorder.Subscribe(t.Context()))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := recorder.Drain(t.Context())

			require.NoError(t, err)
		})
	}
}
