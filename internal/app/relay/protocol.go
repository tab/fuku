package relay

import "time"

// MessageType represents the type of message in the wire protocol
type MessageType string

// Message types for the wire protocol
const (
	// MessageSubscribe is sent from client to server to subscribe to services
	MessageSubscribe MessageType = "subscribe"
	// MessageLog is sent from server to client with log data
	MessageLog MessageType = "log"
	// MessageStatus is sent from server to client after subscribe with connection metadata
	MessageStatus MessageType = "status"
)

// SubscribeRequest is sent from client to server to subscribe to log streams
type SubscribeRequest struct {
	Type     MessageType   `json:"type"`
	Services []string      `json:"services"`           // empty = all services
	Tail     int           `json:"tail,omitempty"`     // 0 = the whole buffered history
	Since    time.Duration `json:"since,omitempty"`    // 0 = no lower bound on buffered time
	NoFollow bool          `json:"noFollow,omitempty"` // true = close after the history is replayed
}

// LogMessage is sent from server to client with log data
type LogMessage struct {
	Type      MessageType `json:"type"`
	Service   string      `json:"service"`
	Message   string      `json:"message"`
	Timestamp time.Time   `json:"timestamp"`
}

// StatusMessage is sent from server to client after subscribe with connection metadata
type StatusMessage struct {
	Type     MessageType `json:"type"`
	Version  string      `json:"version"`
	Profile  string      `json:"profile"`
	Project  string      `json:"project"` // fingerprint of the project directory the instance serves
	Services []string    `json:"services"`
}

// MessageEnvelope is used for type-based message dispatching
type MessageEnvelope struct {
	Type MessageType `json:"type"`
}
