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

func Test_SubscribeRequest_MarshalUnmarshal(t *testing.T) {
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
	tests := []struct {
		name     string
		message  StatusMessage
		expected string
	}{
		{
			name: "with services",
			message: StatusMessage{
				Type:     MessageStatus,
				Version:  "0.17.0",
				Profile:  "default",
				Services: []string{"api", "web"},
			},
			expected: `{"type":"status","version":"0.17.0","profile":"default","services":["api","web"]}`,
		},
		{
			name: "empty services",
			message: StatusMessage{
				Type:    MessageStatus,
				Version: "0.17.0",
				Profile: "core",
			},
			expected: `{"type":"status","version":"0.17.0","profile":"core","services":null}`,
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
