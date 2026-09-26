package readiness

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"time"

	"fuku/internal/adapters/process"
	"fuku/internal/contracts"
)

// checkLog scans both streams until a line matches, the timeout elapses, ctx ends or the child exits, then closes them
func (c *Checker) checkLog(ctx context.Context, pattern string, stdout, stderr io.ReadCloser, timeout time.Duration, done <-chan struct{}) error {
	// sync: deferred first so they run after ended closes, and a scanner woken by the close stays quiet
	defer stdout.Close()
	defer stderr.Close()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}

	matched := make(chan struct{}, 1)
	ended := make(chan struct{})
	deadline := time.Now().Add(timeout)

	defer close(ended)

	scanStream := func(reader io.ReadCloser) {
		defer reader.Close()

		scanner := bufio.NewScanner(reader)
		scanner.Buffer(nil, process.MaxLineSize)

		for scanner.Scan() {
			if c.isDone(ended) {
				return
			}

			if re.MatchString(scanner.Text()) {
				select {
				case matched <- struct{}{}:
				default:
				}

				return
			}
		}

		if err := scanner.Err(); err != nil && !c.isDone(ended) {
			c.log.Warn("Log readiness scan ended before a match", "error", err)
		}
	}

	go scanStream(stdout)
	go scanStream(stderr)

	ctx, cancel := c.contextWithDone(ctx, done)
	defer cancel()

	duration := max(time.Until(deadline), 0)

	select {
	case <-matched:
		return nil
	case <-ctx.Done():
		if c.isDone(done) {
			return contracts.ErrProcessExited
		}

		return ctx.Err()
	case <-time.After(duration):
		return fmt.Errorf("%w: log pattern check after %v", contracts.ErrReadinessTimeout, timeout)
	}
}
