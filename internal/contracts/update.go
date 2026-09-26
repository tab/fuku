package contracts

// EventUpdateAvailable carries an UpdateAvailable
const EventUpdateAvailable MessageType = "update_available"

// UpdateAvailable indicates a newer release is available
type UpdateAvailable struct {
	Version string
}
