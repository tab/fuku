package contracts

import "time"

// API server event types
const (
	EventAPIStarted   MessageType = "api_started"
	EventAPIStopped   MessageType = "api_stopped"
	EventAPIRequested MessageType = "api_request" // frozen wire value, log consumers read it
)

// APIStarted indicates the API server has started listening
type APIStarted struct {
	Listen string
}

// APIStopped indicates the API server has shut down
type APIStopped struct{}

// APIRequested contains metrics for a completed API request
type APIRequested struct {
	Method   string
	Path     string
	Route    string // the matched mux pattern, bounded for metric tags
	Status   int
	Duration time.Duration
}
