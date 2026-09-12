package relay

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_MessageType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		msgType  MessageType
		expected string
	}{
		{
			name:     "subscribe",
			msgType:  MessageSubscribe,
			expected: "subscribe",
		},
		{
			name:     "log",
			msgType:  MessageLog,
			expected: "log",
		},
		{
			name:     "status",
			msgType:  MessageStatus,
			expected: "status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.msgType))
		})
	}
}

func Test_ReplayOptions_bounded(t *testing.T) {
	tail := 10

	tests := []struct {
		name     string
		options  ReplayOptions
		expected bool
	}{
		{
			name:     "no options",
			options:  ReplayOptions{},
			expected: false,
		},
		{
			name:     "tail only",
			options:  ReplayOptions{Tail: &tail},
			expected: true,
		},
		{
			name:     "no-follow only",
			options:  ReplayOptions{NoFollow: true},
			expected: true,
		},
		{
			name:     "tail and no-follow",
			options:  ReplayOptions{Tail: &tail, NoFollow: true},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.options.bounded())
		})
	}
}

func Test_ReplayOptions_equal(t *testing.T) {
	ten := 10
	anotherTen := 10
	twenty := 20

	tests := []struct {
		name     string
		options  ReplayOptions
		other    ReplayOptions
		expected bool
	}{
		{
			name:     "both empty",
			options:  ReplayOptions{},
			other:    ReplayOptions{},
			expected: true,
		},
		{
			name:     "same tail through different pointers",
			options:  ReplayOptions{Tail: &ten, NoFollow: true},
			other:    ReplayOptions{Tail: &anotherTen, NoFollow: true},
			expected: true,
		},
		{
			name:     "different tail",
			options:  ReplayOptions{Tail: &ten},
			other:    ReplayOptions{Tail: &twenty},
			expected: false,
		},
		{
			name:     "tail missing on one side",
			options:  ReplayOptions{Tail: &ten},
			other:    ReplayOptions{},
			expected: false,
		},
		{
			name:     "different no-follow",
			options:  ReplayOptions{NoFollow: true},
			other:    ReplayOptions{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.options.equal(tt.other))
		})
	}
}

func Test_SubscribeRequest_MarshalUnmarshal(t *testing.T) {
	tail := 100
	zeroTail := 0

	tests := []struct {
		name     string
		request  SubscribeRequest
		expected string
	}{
		{
			name: "with services",
			request: SubscribeRequest{
				Type:     MessageSubscribe,
				Services: []string{"api", "web"},
			},
			expected: `{"type":"subscribe","services":["api","web"]}`,
		},
		{
			name: "empty services",
			request: SubscribeRequest{
				Type:     MessageSubscribe,
				Services: []string{},
			},
			expected: `{"type":"subscribe","services":[]}`,
		},
		{
			name: "nil services",
			request: SubscribeRequest{
				Type: MessageSubscribe,
			},
			expected: `{"type":"subscribe","services":null}`,
		},
		{
			name: "bounded read options",
			request: SubscribeRequest{
				Type:          MessageSubscribe,
				Services:      []string{"api"},
				ReplayOptions: ReplayOptions{Tail: &tail, NoFollow: true},
			},
			expected: `{"type":"subscribe","services":["api"],"tail":100,"noFollow":true}`,
		},
		{
			name: "explicit zero tail differs from an omitted one",
			request: SubscribeRequest{
				Type:          MessageSubscribe,
				Services:      []string{"api"},
				ReplayOptions: ReplayOptions{Tail: &zeroTail},
			},
			expected: `{"type":"subscribe","services":["api"],"tail":0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.request)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))

			var decoded SubscribeRequest

			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, tt.request.Type, decoded.Type)
			assert.Equal(t, tt.request.Services, decoded.Services)
			assert.Equal(t, tt.request.Tail, decoded.Tail)
			assert.Equal(t, tt.request.NoFollow, decoded.NoFollow)
		})
	}
}

func Test_LogMessage_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		message  LogMessage
		expected string
	}{
		{
			name: "standard log message",
			message: LogMessage{
				Type:    MessageLog,
				Service: "api",
				Message: "server started on :8080",
			},
			expected: `{"type":"log","service":"api","message":"server started on :8080"}`,
		},
		{
			name: "empty fields",
			message: LogMessage{
				Type: MessageLog,
			},
			expected: `{"type":"log","service":"","message":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.message)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))

			var decoded LogMessage

			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, tt.message, decoded)
		})
	}
}

func Test_StatusMessage_MarshalUnmarshal(t *testing.T) {
	tail := 100

	tests := []struct {
		name     string
		message  StatusMessage
		expected string
	}{
		{
			name: "with services",
			message: StatusMessage{
				Type:        MessageStatus,
				Version:     "0.17.0",
				Instance:    "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
				Fingerprint: "3f2a9c1d8b4e6072",
				Profile:     "default",
				Services:    []string{"api", "web"},
			},
			expected: `{"type":"status","version":"0.17.0","instance":"1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24","fingerprint":"3f2a9c1d8b4e6072","profile":"default","services":["api","web"]}`,
		},
		{
			name: "empty services",
			message: StatusMessage{
				Type:        MessageStatus,
				Version:     "0.17.0",
				Instance:    "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
				Fingerprint: "3f2a9c1d8b4e6072",
				Profile:     "core",
			},
			expected: `{"type":"status","version":"0.17.0","instance":"1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24","fingerprint":"3f2a9c1d8b4e6072","profile":"core","services":null}`,
		},
		{
			name: "empty identity",
			message: StatusMessage{
				Type:     MessageStatus,
				Version:  "0.17.0",
				Profile:  "core",
				Services: []string{"api"},
			},
			expected: `{"type":"status","version":"0.17.0","instance":"","fingerprint":"","profile":"core","services":["api"]}`,
		},
		{
			name: "echoed bounded read options",
			message: StatusMessage{
				Type:          MessageStatus,
				Version:       "0.17.0",
				Profile:       "core",
				Services:      []string{"api"},
				ReplayOptions: ReplayOptions{Tail: &tail, NoFollow: true},
			},
			expected: `{"type":"status","version":"0.17.0","instance":"","fingerprint":"","profile":"core","services":["api"],"tail":100,"noFollow":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.message)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))

			var decoded StatusMessage

			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, tt.message, decoded)
		})
	}
}

func Test_MessageEnvelope_Unmarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected MessageType
	}{
		{
			name:     "subscribe type",
			input:    `{"type":"subscribe","services":["api"]}`,
			expected: MessageSubscribe,
		},
		{
			name:     "log type",
			input:    `{"type":"log","service":"api","message":"hello"}`,
			expected: MessageLog,
		},
		{
			name:     "status type",
			input:    `{"type":"status","version":"0.17.0"}`,
			expected: MessageStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var envelope MessageEnvelope

			err := json.Unmarshal([]byte(tt.input), &envelope)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, envelope.Type)
		})
	}
}
