package cli

import "context"

// Cleaner stops the processes a profile left behind
type Cleaner interface {
	Cleanup(ctx context.Context, profile string) error
}

// Stop kills the processes in the service directories of a profile
type Stop struct {
	options *Options
	cleaner Cleaner
}

// NewStop creates the stop command
func NewStop(options *Options, cleaner Cleaner) *Stop {
	return &Stop{options: options, cleaner: cleaner}
}

// Run cleans up the profile and returns the exit code
func (s *Stop) Run(ctx context.Context) (int, error) {
	profile := s.options.Profile

	if err := s.cleaner.Cleanup(ctx, profile); err != nil {
		return 1, err
	}

	return 0, nil
}
