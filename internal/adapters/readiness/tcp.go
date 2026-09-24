package readiness

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// probeTimeout bounds the dial that checks whether a service address is already taken before launch
const probeTimeout = 100 * time.Millisecond

// Default ports of the URL schemes a readiness probe understands
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
	portHTTP    = "80"
	portHTTPS   = "443"
)

// checkTCP dials address until it accepts a connection, the timeout elapses, ctx is cancelled or the process exits
func (c *Checker) checkTCP(ctx context.Context, address string, timeout, interval time.Duration, done <-chan struct{}) error {
	deadline := time.Now().Add(timeout)

	ctx, cancel := c.contextWithDone(ctx, done)
	defer cancel()

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("%w: TCP check after %v", contracts.ErrReadinessTimeout, timeout)
		}

		conn, err := net.DialTimeout("tcp", address, min(interval, remaining))
		if err == nil {
			conn.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			if c.isDone(done) {
				return contracts.ErrProcessExited
			}

			return ctx.Err()
		case <-time.After(min(interval, time.Until(deadline))):
		}
	}
}

// ProbePort reports whether the probe's address is in use through the one port probe
func (c *Checker) ProbePort(readiness model.Readiness) model.Port {
	return ProbePort(readiness)
}

// ProbePort dials the probe's address and reports whether it is in use (a probe without an address never is)
func ProbePort(readiness model.Readiness) model.Port {
	address := extractAddress(readiness)
	if address == "" {
		return model.Port{}
	}

	conn, err := net.DialTimeout("tcp", address, probeTimeout)
	if err != nil {
		return model.Port{Address: address}
	}

	conn.Close()

	return model.Port{Address: address, InUse: true}
}

// extractAddress returns the host:port a readiness probe listens on, or empty when it names none
func extractAddress(readiness model.Readiness) string {
	switch readiness.Type {
	case model.ReadinessHTTP:
		return extractFromURL(readiness.URL)
	case model.ReadinessTCP:
		return readiness.Address
	default:
		return ""
	}
}

// extractFromURL extracts host:port from URL (e.g., "http://localhost:8080/health" -> "localhost:8080")
func extractFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	host := parsed.Hostname()
	if host == "" {
		return ""
	}

	port := parsed.Port()
	if port == "" {
		switch parsed.Scheme {
		case schemeHTTP:
			port = portHTTP
		case schemeHTTPS:
			port = portHTTPS
		default:
			return ""
		}
	}

	return net.JoinHostPort(host, port)
}
