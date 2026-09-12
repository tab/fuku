package relay

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

// ReplayOptions bounds the buffered replay a client receives (shared by the CLI, the log screen and the wire messages)
type ReplayOptions struct {
	Tail     *int `json:"tail,omitempty"`     // newest n matching messages (nil = full matching history)
	NoFollow bool `json:"noFollow,omitempty"` // close the stream after replay instead of following
}

// bounded reports whether the options ask for a bounded read
func (o ReplayOptions) bounded() bool {
	return o.Tail != nil || o.NoFollow
}

// equal reports whether both options describe the same replay (a nil tail only matches a nil tail)
func (o ReplayOptions) equal(other ReplayOptions) bool {
	if o.NoFollow != other.NoFollow {
		return false
	}

	if o.Tail == nil || other.Tail == nil {
		return o.Tail == nil && other.Tail == nil
	}

	return *o.Tail == *other.Tail
}

// SubscribeRequest is sent from client to server to subscribe to log streams
type SubscribeRequest struct {
	Type     MessageType `json:"type"`
	Services []string    `json:"services"` // empty = all services
	ReplayOptions
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
	ReplayOptions
}

// MessageEnvelope is used for type-based message dispatching
type MessageEnvelope struct {
	Type MessageType `json:"type"`
}
