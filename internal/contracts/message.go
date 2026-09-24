package contracts

import "time"

// MessageType identifies a bus message (event or command)
type MessageType string

// Critical reports whether the type must reach every required subscriber
func (t MessageType) Critical() bool {
	return critical[t]
}

// critical lists every message type with its delivery class
var critical = map[MessageType]bool{
	EventCommandStarted:          false,
	EventPhaseChanged:            true,
	EventProfileResolved:         true,
	EventPreflightStarted:        true,
	EventPreflightKilled:         false,
	EventPreflightComplete:       true,
	EventTierStarting:            true,
	EventTierReady:               true,
	EventServiceStarting:         true,
	EventReadinessComplete:       false,
	EventServiceReady:            true,
	EventServiceFailed:           true,
	EventServiceStopping:         true,
	EventServiceStopped:          true,
	EventServiceRestarting:       true,
	EventSignalReceived:          true,
	EventWatchTriggered:          true,
	EventWatchStarted:            false,
	EventWatchStopped:            false,
	EventResourceSampled:         false,
	EventServiceResourcesSampled: false,
	EventAPIStarted:              true,
	EventAPIStopped:              true,
	EventAPIRequested:            false,
	EventUpdateAvailable:         false,
	EventSnapshotChanged:         false,
	CommandStartService:          true,
	CommandStopService:           true,
	CommandRestartService:        true,
	CommandStopAll:               true,
}

// Message represents a bus message (event or command)
type Message struct {
	Type      MessageType
	Timestamp time.Time
	Data      any
}
