package logs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_MessageType_Constants(t *testing.T) {
	assert.Equal(t, MessageType("subscribe"), MessageSubscribe)
	assert.Equal(t, MessageType("log"), MessageLog)
}

func Test_SubscribeRequest_Marshal(t *testing.T) {
	tests := []struct {
		name     string
		request  SubscribeRequest
		contains []string
	}{
		{name: "With services", request: SubscribeRequest{Type: MessageSubscribe, Services: []string{"api", "db"}}, contains: []string{`"type":"subscribe"`, `"services":["api","db"]`}},
		{name: "Empty services", request: SubscribeRequest{Type: MessageSubscribe, Services: nil}, contains: []string{`"type":"subscribe"`, `"services":null`}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.request)
			assert.NoError(t, err)

			for _, c := range tt.contains {
				assert.Contains(t, string(data), c)
			}
		})
	}
}

func Test_SubscribeRequest_Unmarshal(t *testing.T) {
	data := `{"type":"subscribe","services":["api"]}`

	var req SubscribeRequest

	err := json.Unmarshal([]byte(data), &req)
	assert.NoError(t, err)
	assert.Equal(t, MessageSubscribe, req.Type)
	assert.Equal(t, []string{"api"}, req.Services)
}

func Test_LogMessage_Marshal(t *testing.T) {
	msg := LogMessage{Type: MessageLog, Service: "api", Message: "Hello World"}

	data, err := json.Marshal(msg)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"type":"log"`)
	assert.Contains(t, string(data), `"service":"api"`)
	assert.Contains(t, string(data), `"message":"Hello World"`)
}

func Test_LogMessage_Unmarshal(t *testing.T) {
	data := `{"type":"log","service":"db","message":"Connected"}`

	var msg LogMessage

	err := json.Unmarshal([]byte(data), &msg)
	assert.NoError(t, err)
	assert.Equal(t, MessageLog, msg.Type)
	assert.Equal(t, "db", msg.Service)
	assert.Equal(t, "Connected", msg.Message)
}
