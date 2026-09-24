package bus

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/contracts"
)

func Test_Subscriber_Matches(t *testing.T) {
	tests := []struct {
		name     string
		types    []contracts.MessageType
		msgType  contracts.MessageType
		expected bool
	}{
		{
			name:     "nil filter receives every type",
			types:    nil,
			msgType:  contracts.EventResourceSampled,
			expected: true,
		},
		{
			name:     "listed type",
			types:    []contracts.MessageType{contracts.CommandStopAll, contracts.EventWatchTriggered},
			msgType:  contracts.EventWatchTriggered,
			expected: true,
		},
		{
			name:     "unlisted type",
			types:    []contracts.MessageType{contracts.CommandStopAll},
			msgType:  contracts.EventPhaseChanged,
			expected: false,
		},
		{
			name:     "empty filter receives nothing",
			types:    []contracts.MessageType{},
			msgType:  contracts.EventPhaseChanged,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := newSubscriber(contracts.SubscribeOptions{Name: "store", Types: tt.types}, 1)

			assert.Equal(t, tt.expected, sub.matches(tt.msgType))
		})
	}
}

func Test_Subscriber_Send(t *testing.T) {
	tests := []struct {
		name          string
		before        func(sub *subscriber)
		expectedQueue int
		expectedDrops map[contracts.MessageType]uint64
		expectedFull  bool
	}{
		{
			name:          "queues while a slot is free",
			before:        func(*subscriber) {},
			expectedQueue: 1,
			expectedDrops: map[contracts.MessageType]uint64{},
			expectedFull:  false,
		},
		{
			name: "counts a drop when full",
			before: func(sub *subscriber) {
				sub.send(contracts.Message{Type: contracts.EventPhaseChanged})
				sub.send(contracts.Message{Type: contracts.EventPhaseChanged})
			},
			expectedQueue: 2,
			expectedDrops: map[contracts.MessageType]uint64{contracts.EventPhaseChanged: 1},
			expectedFull:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := newSubscriber(contracts.SubscribeOptions{Name: "store", Required: true}, 2)

			tt.before(sub)

			sub.send(contracts.Message{Type: contracts.EventPhaseChanged})

			assert.Len(t, sub.Messages(), tt.expectedQueue)
			assert.Equal(t, tt.expectedFull, sub.full())
			assert.Equal(t, tt.expectedDrops, sub.close())
		})
	}
}

func Test_Subscriber_Close(t *testing.T) {
	sub := newSubscriber(contracts.SubscribeOptions{Name: "store"}, 1)

	sub.send(contracts.Message{Type: contracts.EventPhaseChanged})

	first := sub.close()

	queued, open := <-sub.Messages()
	_, reopened := <-sub.Messages()

	assert.Equal(t, map[contracts.MessageType]uint64{}, first)
	assert.Equal(t, contracts.EventPhaseChanged, queued.Type)
	assert.True(t, open, "a queued message is still delivered after close")
	assert.False(t, reopened)
}
