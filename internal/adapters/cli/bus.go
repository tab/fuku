package cli

import (
	"context"

	"fuku/internal/contracts"
)

// Announcer publishes the command the process runs
type Announcer struct {
	options   *Options
	publisher contracts.Publisher
}

// NewAnnouncer creates the announcer of the parsed command
func NewAnnouncer(options *Options, publisher contracts.Publisher) *Announcer {
	return &Announcer{options: options, publisher: publisher}
}

// Start publishes CommandStarted for the parsed command, except in a detached child, whose parent announced it
func (a *Announcer) Start(context.Context) error {
	if a.options.DetachedChild {
		return nil
	}

	//nolint:errcheck // a non-critical publish never fails
	a.publisher.Publish(contracts.Message{
		Type: contracts.EventCommandStarted,
		Data: contracts.CommandStarted{
			Command: a.options.Type.String(),
			Profile: a.options.Profile,
			UI:      !a.options.NoUI,
		},
	})

	return nil
}

// Stop has nothing to release
func (a *Announcer) Stop(context.Context) error {
	return nil
}
