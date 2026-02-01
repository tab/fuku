package logs

// MessageType represents the type of message in the wire protocol
type MessageType string

// Message types for the wire protocol
const (
	// MessageSubscribe is sent from client to server to subscribe to services
	MessageSubscribe MessageType = "subscribe"
	// MessageLog is sent from server to client with log data
	MessageLog MessageType = "log"
)

// SubscribeRequest is sent from client to server to subscribe to log streams
type SubscribeRequest struct {
	Type     MessageType `json:"type"`
	Services []string    `json:"services"` // empty = all services
}

// LogMessage is sent from server to client with log data
type LogMessage struct {
	Type    MessageType `json:"type"`
	Service string      `json:"service"`
	Message string      `json:"message"`
}
