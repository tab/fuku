package detach

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// queue is a Subscription over a plain channel
type queue chan contracts.Message

func (q queue) Messages() <-chan contracts.Message {
	return q
}

func Test_Progress_Subscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	options := contracts.SubscribeOptions{Name: "detach", Required: true, Types: progressTypes}

	tests := []struct {
		name     string
		before   func()
		expected error
	}{
		{
			name: "registers the required detach subscription",
			before: func() {
				messages := make(queue)
				close(messages)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), options).Return(messages, nil)
			},
		},
		{
			name: "returns the subscribe error",
			before: func() {
				mockSubscriber.EXPECT().Subscribe(gomock.Any(), options).Return(nil, contracts.ErrBusClosed)
			},
			expected: contracts.ErrBusClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := NewProgress(mockSubscriber, nil, nil, nil, nil, nil).Subscribe(t.Context())

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Progress_Drain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockOutput := NewMockOutput(ctrl)

	messages := make(queue, 1)
	messages <- contracts.Message{
		Type: contracts.EventServiceStarting,
		Data: contracts.ServiceStarting{ServiceEvent: contracts.ServiceEvent{Service: model.Service{Name: "api"}}},
	}

	mockSubscriber.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(messages, nil)
	mockOutput.EXPECT().Write(encode(Record{Kind: KindStarting, Service: "api"})).Return(0, nil)

	progress := NewProgress(mockSubscriber, nil, nil, nil, mockOutput, nil)

	require.NoError(t, progress.Subscribe(t.Context()))
	require.NoError(t, progress.Drain(t.Context()))
}

func Test_Progress_handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRelay := NewMockRelay(ctrl)
	mockAPI := NewMockAPI(ctrl)
	mockReporter := NewMockReporter(ctrl)
	mockOutput := NewMockOutput(ctrl)
	mockLog := NewMockLogger(ctrl)

	api := model.Service{ID: "api-id", Name: "api"}
	web := model.Service{ID: "web-id", Name: "web"}
	event := func(svc model.Service) contracts.ServiceEvent {
		return contracts.ServiceEvent{Service: svc, Tier: "platform"}
	}
	expectFail := func(expected string) {
		mockReporter.EXPECT().Fail(gomock.Any()).Do(func(err error) {
			assert.EqualError(t, err, expected)
		})
	}
	bindErr := errors.New("socket is already in use")
	running := contracts.Message{
		Type: contracts.EventPhaseChanged,
		Data: contracts.PhaseChanged{Phase: model.PhaseRunning, Duration: 4 * time.Second, ServiceCount: 2},
	}

	tests := []struct {
		name     string
		before   func()
		api      API
		messages []contracts.Message
	}{
		{
			name: "lists the services of the resolved profile",
			before: func() {
				mockOutput.EXPECT().Write(encode(Record{Kind: KindProfile, Services: []string{"api", "web"}})).Return(0, nil)
			},
			messages: []contracts.Message{{
				Type: contracts.EventProfileResolved,
				Data: contracts.ProfileResolved{Tiers: model.Tiers{
					{Name: "platform", Services: []*model.Service{&api}},
					{Name: "edge", Services: []*model.Service{&web}},
				}},
			}},
		},
		{
			name: "reports a ready service with its startup time",
			before: func() {
				mockOutput.EXPECT().Write(encode(Record{Kind: KindReady, Service: "api", Duration: time.Second})).Return(0, nil)
			},
			messages: []contracts.Message{{
				Type: contracts.EventServiceReady,
				Data: contracts.ServiceReady{ServiceEvent: event(api), Duration: time.Second},
			}},
		},
		{
			name: "a failed service fails the run and settles the start",
			before: func() {
				mockOutput.EXPECT().Write(encode(Record{Kind: KindFailed, Service: "api", Error: "max retries exceeded"})).Return(0, nil)
				expectFail("service 'api' failed to start: max retries exceeded")
			},
			messages: []contracts.Message{
				{
					Type: contracts.EventServiceFailed,
					Data: contracts.ServiceFailed{ServiceEvent: event(api), Error: errors.New("max retries exceeded")},
				},
				running,
			},
		},
		{
			name: "an unexpected stop fails the run",
			before: func() {
				mockOutput.EXPECT().Write(encode(Record{Kind: KindFailed, Service: "web", Error: contracts.ErrUnexpectedExit.Error()})).Return(0, nil)
				expectFail("service 'web' failed to start: " + contracts.ErrUnexpectedExit.Error())
			},
			messages: []contracts.Message{{
				Type: contracts.EventServiceStopped,
				Data: contracts.ServiceStopped{ServiceEvent: event(web), Unexpected: true},
			}},
		},
		{
			name:   "an expected stop is not reported",
			before: func() {},
			messages: []contracts.Message{{
				Type: contracts.EventServiceStopped,
				Data: contracts.ServiceStopped{ServiceEvent: event(web)},
			}},
		},
		{
			name: "the running phase reports success once the socket is bound and releases the output",
			before: func() {
				gomock.InOrder(
					mockRelay.EXPECT().Bound(gomock.Any()).Return(nil),
					mockOutput.EXPECT().Write(encode(Record{Kind: KindRunning, PID: os.Getpid(), Count: 2, Duration: 4 * time.Second})).Return(0, nil),
					mockOutput.EXPECT().Release().Return(nil),
				)
			},
			messages: []contracts.Message{running, running},
		},
		{
			name: "the running record carries the API address once the API is bound",
			before: func() {
				gomock.InOrder(
					mockRelay.EXPECT().Bound(gomock.Any()).Return(nil),
					mockAPI.EXPECT().Address(gomock.Any()).Return("127.0.0.1:9090"),
					mockOutput.EXPECT().Write(encode(Record{Kind: KindRunning, PID: os.Getpid(), Count: 2, Duration: 4 * time.Second, Address: "127.0.0.1:9090"})).Return(0, nil),
					mockOutput.EXPECT().Release().Return(nil),
				)
			},
			api:      mockAPI,
			messages: []contracts.Message{running},
		},
		{
			name: "a socket that failed to bind fails the run",
			before: func() {
				mockRelay.EXPECT().Bound(gomock.Any()).Return(bindErr)
				expectFail("failed to start the logs server: socket is already in use")
			},
			messages: []contracts.Message{running},
		},
		{
			name:   "another phase is not reported",
			before: func() {},
			messages: []contracts.Message{{
				Type: contracts.EventPhaseChanged,
				Data: contracts.PhaseChanged{Phase: model.PhaseStartup},
			}},
		},
		{
			name: "a failed write is logged",
			before: func() {
				mockOutput.EXPECT().Write(gomock.Any()).Return(0, io.ErrClosedPipe)
				mockLog.EXPECT().Warn("Failed to report the detached start", "error", io.ErrClosedPipe)
			},
			messages: []contracts.Message{{
				Type: contracts.EventServiceStarting,
				Data: contracts.ServiceStarting{ServiceEvent: event(api)},
			}},
		},
		{
			name: "a failed release fails the run",
			before: func() {
				mockRelay.EXPECT().Bound(gomock.Any()).Return(nil)
				mockOutput.EXPECT().Write(gomock.Any()).Return(0, nil)
				mockOutput.EXPECT().Release().Return(os.ErrClosed)
				expectFail("failed to release the detached start output: file already closed")
			},
			messages: []contracts.Message{running},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			progress := NewProgress(nil, mockRelay, tt.api, mockReporter, mockOutput, mockLog)

			for _, msg := range tt.messages {
				progress.handle(context.Background(), msg)
			}
		})
	}
}
