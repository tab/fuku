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

// Stop stops the running fuku of the project, then kills the processes in the service directories of a profile
type Stop struct {
	options  *Options
	instance Instance
	cleaner  Cleaner
}

// NewStop creates the stop command
func NewStop(options *Options, instance Instance, cleaner Cleaner) *Stop {
	return &Stop{options: options, instance: instance, cleaner: cleaner}
}

// Run stops the running instance, cleans up the profile and returns the exit code
func (s *Stop) Run(ctx context.Context) (int, error) {
	profile := s.options.Profile

	if err := s.instance.Stop(ctx); err != nil {
		return 1, err
	}

	if err := s.cleaner.Cleanup(ctx, profile); err != nil {
		return 1, err
	}

	return 0, nil
}
