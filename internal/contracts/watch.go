package contracts

import "fuku/internal/model"

// Watch event types
const (
	EventWatchTriggered MessageType = "watch_triggered"
	EventWatchStarted   MessageType = "watch_started"
	EventWatchStopped   MessageType = "watch_stopped"
)

// WatchTriggered indicates file changes detected for a watched service
type WatchTriggered struct {
	Service      model.Service
	ChangedFiles []string
}

// WatchStarted indicates the watcher now watches the files of a service
type WatchStarted struct {
	Service model.Service
}

// WatchStopped indicates the watcher no longer watches the files of a service
type WatchStopped struct {
	Service model.Service
}
