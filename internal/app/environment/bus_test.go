package environment

import (
	"testing"

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
	mockReader := NewMockReader(ctrl)

	subject := NewStore(mockSubscriber, mockReader)

	subscription := contracts.SubscribeOptions{
		Name:  "environment",
		Types: []contracts.MessageType{contracts.EventProfileResolved, contracts.EventServiceStarting},
	}

	tests := []struct {
		name     string
		before   func()
		expected error
	}{
		{
			name: "registers the optional lifecycle subscription",
			before: func() {
				messages := make(queue)
				close(messages)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), subscription).Return(messages, nil)
			},
		},
		{
			name: "returns the subscribe error",
			before: func() {
				mockSubscriber.EXPECT().Subscribe(gomock.Any(), subscription).Return(nil, contracts.ErrBusClosed)
			},
			expected: contracts.ErrBusClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := subject.Subscribe(t.Context())

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Store_Drain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockReader := NewMockReader(ctrl)

	tests := []struct {
		name   string
		before func() *Store
	}{
		{
			name: "a store that never subscribed has nothing to drain",
			before: func() *Store {
				return NewStore(mockSubscriber, mockReader)
			},
		},
		{
			name: "a subscribed store drains once its closed queue ends the loop",
			before: func() *Store {
				s := NewStore(mockSubscriber, mockReader)

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

func Test_Store_handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockReader := NewMockReader(ctrl)

	envFiles := &model.EnvFiles{Files: []string{".env"}}

	starting := contracts.Message{
		Type: contracts.EventServiceStarting,
		Data: contracts.ServiceStarting{
			ServiceEvent: contracts.ServiceEvent{Service: model.Service{ID: "id-api", Name: "api", Directory: "svc/api", Environment: envFiles}, Tier: "foundation"},
		},
	}

	cached := []model.Env{{Key: "KEEP", Value: "1"}}

	tests := []struct {
		name     string
		before   func(subject *Store)
		msg      contracts.Message
		expected map[string][]model.Env
	}{
		{
			name: "profile resolved seeds every service",
			before: func(*Store) {
				mockReader.EXPECT().Read("svc/api", ".env").Return([]model.Env{{Key: "A", Value: "1"}}, nil)
				mockReader.EXPECT().Read("svc/web", ".env").Return([]model.Env{{Key: "B", Value: "2"}}, nil)
			},
			msg: contracts.Message{
				Type: contracts.EventProfileResolved,
				Data: contracts.ProfileResolved{
					Tiers: []model.Tier{
						{Name: "foundation", Services: []*model.Service{
							{ID: "id-api", Name: "api", Directory: "svc/api", Environment: envFiles},
							{ID: "id-web", Name: "web", Directory: "svc/web", Environment: envFiles},
						}},
					},
				},
			},
			expected: map[string][]model.Env{
				"id-api": {{Key: "A", Value: "1"}},
				"id-web": {{Key: "B", Value: "2"}},
			},
		},
		{
			name: "service starting re-reads the files of that service",
			before: func(subject *Store) {
				subject.cache["id-api"] = cached

				mockReader.EXPECT().Read("svc/api", ".env").Return([]model.Env{{Key: "VERSION", Value: "2"}}, nil)
			},
			msg: starting,
			expected: map[string][]model.Env{
				"id-api": {{Key: "VERSION", Value: "2"}},
			},
		},
		{
			name: "profile resolved with a foreign payload is ignored",
			before: func(subject *Store) {
				subject.cache["id-api"] = cached
			},
			msg: contracts.Message{
				Type: contracts.EventProfileResolved,
				Data: contracts.ServiceStarting{},
			},
			expected: map[string][]model.Env{
				"id-api": cached,
			},
		},
		{
			name: "service starting with a foreign payload is ignored",
			before: func(subject *Store) {
				subject.cache["id-api"] = cached
			},
			msg: contracts.Message{
				Type: contracts.EventServiceStarting,
				Data: contracts.ProfileResolved{},
			},
			expected: map[string][]model.Env{
				"id-api": cached,
			},
		},
		{
			name: "unrelated events are ignored",
			before: func(subject *Store) {
				subject.cache["id-api"] = cached
			},
			msg: contracts.Message{Type: contracts.EventTierReady},
			expected: map[string][]model.Env{
				"id-api": cached,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject := NewStore(mockSubscriber, mockReader)
			tt.before(subject)

			subject.handle(tt.msg)

			for id, entries := range tt.expected {
				assert.Equal(t, entries, subject.Env(id), id)
			}
		})
	}
}
