package tui

import "time"

// Options is the profile the services view runs and the retry policy its health tab shows
type Options struct {
	Profile       string
	RetryAttempts int
	RetryBackoff  time.Duration
}
