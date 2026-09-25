package updater

import (
	"context"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// ReleaseSource returns the latest published release
type ReleaseSource interface {
	Latest(ctx context.Context) (model.Release, error)
}

// Logger is the logging surface the version checker writes through
type Logger interface {
	Debug(msg string, args ...any)
}

// Checker performs a one-shot version check and publishes EventUpdateAvailable when a newer release exists
type Checker struct {
	options   Options
	source    ReleaseSource
	publisher contracts.Publisher
	cancel    context.CancelFunc
	done      chan struct{}
	log       Logger
}

// NewChecker creates a new version checker
func NewChecker(options Options, source ReleaseSource, publisher contracts.Publisher, log Logger) *Checker {
	return &Checker{
		options:   options,
		source:    source,
		publisher: publisher,
		done:      make(chan struct{}),
		log:       log,
	}
}

// Start runs the one-shot check on its own goroutine
func (c *Checker) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)

	go func() {
		defer close(c.done)

		c.run(ctx)
	}()

	return nil
}

// Stop abandons an unfinished check and waits for it to return
func (c *Checker) Stop(ctx context.Context) error {
	c.cancel()

	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run fetches the latest release and publishes EventUpdateAvailable when it is newer than the running version
func (c *Checker) run(ctx context.Context) {
	if !c.options.Enabled {
		return
	}

	release, err := c.source.Latest(ctx)
	if err != nil {
		c.log.Debug("Fetch latest release failed", "error", err)

		return
	}

	if !isNewer(c.options.Version, release.Tag) {
		return
	}

	//nolint:errcheck // a non-critical publish never fails
	c.publisher.Publish(contracts.Message{
		Type: contracts.EventUpdateAvailable,
		Data: contracts.UpdateAvailable{Version: normalize(release.Tag)},
	})
}
