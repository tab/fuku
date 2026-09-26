package services

import "time"

// Options is the profile the runtime runs and the service start retry policy
type Options struct {
	Profile       string
	RetryAttempts int
	RetryBackoff  time.Duration
}
