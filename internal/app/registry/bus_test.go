package registry

import (
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

func Test_Store_Subscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	s := NewStore(mockSubscriber, mockPublisher)

	options := contracts.SubscribeOptions{Name: "registry", Required: true, Types: projected}

	tests := []struct {
		name     string
		before   func()
		expected error
	}{
		{
			name: "registers the required filtered subscription",
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

			err := s.Subscribe(t.Context())

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Store_Subscribe_ProjectsTheQueue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	s := NewStore(mockSubscriber, mockPublisher)

	messages := make(queue, 1)
	messages <- contracts.Message{
		Type: contracts.EventProfileResolved,
		Data: contracts.ProfileResolved{Profile: "default"},
	}

	published := make(chan contracts.Message, 1)
	capture := func(msg contracts.Message) error {
		published <- msg

		return nil
	}

	mockSubscriber.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(messages, nil)
	mockPublisher.EXPECT().Publish(gomock.Any()).DoAndReturn(capture)

	err := s.Subscribe(t.Context())

	require.NoError(t, err)

	msg := <-published

	assert.Equal(t, contracts.Message{Type: contracts.EventSnapshotChanged, Data: contracts.SnapshotChanged{}}, msg)
	assert.Equal(t, "default", s.snapshot.Profile)
}

func Test_Store_Drain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	tests := []struct {
		name   string
		before func() *Store
	}{
		{
			name: "a store that never subscribed has nothing to drain",
			before: func() *Store {
				return NewStore(mockSubscriber, mockPublisher)
			},
		},
		{
			name: "a subscribed store drains once its closed queue ends the loop",
			before: func() *Store {
				s := NewStore(mockSubscriber, mockPublisher)

				messages := make(queue)
				close(messages)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(messages, nil)

				require.NoError(t, s.Subscribe(t.Context()))

				return s
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.before()

			err := s.Drain(t.Context())

			require.NoError(t, err)
		})
	}
}

func Test_Store_Handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	s := NewStore(mockSubscriber, mockPublisher)

	api := contracts.ServiceEvent{Service: model.Service{ID: "test-id-api", Name: "api"}, Tier: "foundation"}
	startedAt := time.Now()
	changed := contracts.Message{Type: contracts.EventSnapshotChanged, Data: contracts.SnapshotChanged{}}

	tests := []struct {
		name   string
		before func()
		msg    contracts.Message
	}{
		{
			name: "a resolved profile is announced",
			before: func() {
				mockPublisher.EXPECT().Publish(changed).Return(nil)
			},
			msg: contracts.Message{
				Type: contracts.EventProfileResolved,
				Data: contracts.ProfileResolved{
					Profile: "default",
					Tiers:   []model.Tier{{Name: "foundation", Services: []*model.Service{&api.Service}}},
				},
			},
		},
		{
			name: "a phase change is announced",
			before: func() {
				mockPublisher.EXPECT().Publish(changed).Return(nil)
			},
			msg: contracts.Message{
				Timestamp: startedAt,
				Type:      contracts.EventPhaseChanged,
				Data:      contracts.PhaseChanged{Phase: model.PhaseStartup},
			},
		},
		{
			name:   "the same phase again is a no-op",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventPhaseChanged,
				Data: contracts.PhaseChanged{Phase: model.PhaseStartup},
			},
		},
		{
			name:   "malformed data is a no-op",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventServiceStarting,
				Data: "not a service event",
			},
		},
		{
			name:   "an event the read model does not project is a no-op",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventSnapshotChanged,
				Data: contracts.SnapshotChanged{},
			},
		},
		{
			name: "a lifecycle event is announced",
			before: func() {
				mockPublisher.EXPECT().Publish(changed).Return(nil)
			},
			msg: contracts.Message{
				Type: contracts.EventServiceReady,
				Data: contracts.ServiceReady{ServiceEvent: api, PID: 1234, StartedAt: startedAt},
			},
		},
		{
			name: "a resource sample is announced",
			before: func() {
				mockPublisher.EXPECT().Publish(changed).Return(nil)
			},
			msg: contracts.Message{
				Type: contracts.EventServiceResourcesSampled,
				Data: contracts.ServiceResourcesSampled{Services: []contracts.ServiceResourceSample{{ID: "test-id-api", PID: 1234, CPU: 1.5, Memory: 2048}}},
			},
		},
		{
			name:   "an unchanged resource sample is a no-op",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventServiceResourcesSampled,
				Data: contracts.ServiceResourcesSampled{Services: []contracts.ServiceResourceSample{{ID: "test-id-api", PID: 1234, CPU: 1.5, Memory: 2048}}},
			},
		},
		{
			name:   "a sample of another PID is a no-op",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventServiceResourcesSampled,
				Data: contracts.ServiceResourcesSampled{Services: []contracts.ServiceResourceSample{{ID: "test-id-api", PID: 4321, CPU: 9, Memory: 9}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			s.handle(tt.msg)

			assert.True(t, ctrl.Satisfied())
		})
	}
}
