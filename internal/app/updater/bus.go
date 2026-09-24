package updater

import "fuku/internal/contracts"

// publishUpdate announces that a newer release is available
func (c *Checker) publishUpdate(version string) {
	//nolint:errcheck // a non-critical publish never fails
	c.publisher.Publish(contracts.Message{
		Type: contracts.EventUpdateAvailable,
		Data: contracts.UpdateAvailable{Version: version},
	})
}
