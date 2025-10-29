package readiness

import (
	"context"
	"regexp"
	"time"

	"fuku/internal/app/errors"
)

// LogChecker checks service readiness by matching log patterns
type LogChecker struct {
	pattern  *regexp.Regexp
	timeout  time.Duration
	logChan  chan string
	doneChan chan struct{}
}

// NewLogChecker creates a new log pattern readiness checker
func NewLogChecker(pattern string, timeout time.Duration) (*LogChecker, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.ErrInvalidRegexPattern
	}

	return &LogChecker{
		pattern:  re,
		timeout:  timeout,
		logChan:  make(chan string, 100),
		doneChan: make(chan struct{}),
	}, nil
}

// AddLogLine adds a log line to be checked against the pattern
func (l *LogChecker) AddLogLine(line string) {
	select {
	case l.logChan <- line:
	case <-l.doneChan:
	default:
	}
}

// Check waits for a log line matching the pattern
func (l *LogChecker) Check(ctx context.Context) error {
	defer close(l.doneChan)

	deadline := time.NewTimer(l.timeout)
	defer deadline.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.ErrReadinessCheckFailed
		case line := <-l.logChan:
			if l.pattern.MatchString(line) {
				return nil
			}
		}
	}
}
