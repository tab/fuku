package logsocket

import "fuku/internal/model"

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
	Type     MessageType `json:"type"`
	Services []string    `json:"services"` // empty = all services
	model.ReplayOptions
}

// LogMessage is sent from server to client with log data
type LogMessage struct {
	Type    MessageType `json:"type"`
	Service string      `json:"service"`
	Message string      `json:"message"`
}

// StatusMessage is sent from server to client after subscribe with connection metadata
type StatusMessage struct {
	Type        MessageType `json:"type"`
	Version     string      `json:"version"`
	Instance    string      `json:"instance"`
	Fingerprint string      `json:"fingerprint"`
	Profile     string      `json:"profile"`
	Services    []string    `json:"services"`
	PID         int         `json:"pid,omitempty"`
	model.ReplayOptions
}

// MessageEnvelope is used for type-based message dispatching
type MessageEnvelope struct {
	Type MessageType `json:"type"`
}
