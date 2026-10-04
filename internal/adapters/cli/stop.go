package cli

import "context"

// Instance stops the running fuku of the project, if one answers
type Instance interface {
	Stop(ctx context.Context) error
}

// Cleaner stops the processes a profile left behind
type Cleaner interface {
	Cleanup(ctx context.Context, profile string) error
}

// Socket removes the project socket a dead instance left behind
type Socket interface {
	Remove() error
}

// Stop stops the running fuku of the project, kills the processes of a profile and removes the stale socket
type Stop struct {
	options  *Options
	instance Instance
	cleaner  Cleaner
	socket   Socket
}

// NewStop creates the stop command
func NewStop(options *Options, instance Instance, cleaner Cleaner, socket Socket) *Stop {
	return &Stop{options: options, instance: instance, cleaner: cleaner, socket: socket}
}

// Run stops the running instance, cleans up the profile, removes the stale socket and returns the exit code
func (s *Stop) Run(ctx context.Context) (int, error) {
	profile := s.options.Profile

	if err := s.instance.Stop(ctx); err != nil {
		return 1, err
	}

	if err := s.cleaner.Cleanup(ctx, profile); err != nil {
		return 1, err
	}

	if err := s.socket.Remove(); err != nil {
		return 1, err
	}

	return 0, nil
}
