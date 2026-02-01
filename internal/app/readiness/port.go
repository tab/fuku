package readiness

import (
	"net"
	"net/url"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
	portHTTP    = "80"
	portHTTPS   = "443"
)

// ExtractFromURL extracts port from HTTP URL (e.g., "http://localhost:8080/health" → "8080")
func ExtractFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	if port := parsed.Port(); port != "" {
		return port
	}

	switch parsed.Scheme {
	case schemeHTTP:
		return portHTTP
	case schemeHTTPS:
		return portHTTPS
	default:
		return ""
	}
}

// ExtractFromAddress extracts port from TCP address (e.g., "localhost:9090" → "9090")
func ExtractFromAddress(address string) string {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return ""
	}

	return port
}
