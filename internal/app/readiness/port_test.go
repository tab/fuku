package readiness

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ExtractFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "URL with explicit port",
			url:      "http://localhost:8080/health",
			expected: "8080",
		},
		{
			name:     "URL with explicit port no path",
			url:      "http://localhost:8080",
			expected: "8080",
		},
		{
			name:     "HTTP URL without port defaults to 80",
			url:      "http://localhost/health",
			expected: "80",
		},
		{
			name:     "HTTPS URL without port defaults to 443",
			url:      "https://localhost/health",
			expected: "443",
		},
		{
			name:     "URL with IP address and port",
			url:      "http://127.0.0.1:3000/api",
			expected: "3000",
		},
		{
			name:     "URL with high port number",
			url:      "http://localhost:65535",
			expected: "65535",
		},
		{
			name:     "invalid URL returns empty",
			url:      "://invalid",
			expected: "",
		},
		{
			name:     "empty URL returns empty",
			url:      "",
			expected: "",
		},
		{
			name:     "unknown scheme without port returns empty",
			url:      "ftp://localhost/file",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractFromURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_ExtractFromAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected string
	}{
		{
			name:     "address with port",
			address:  "localhost:9090",
			expected: "9090",
		},
		{
			name:     "IP address with port",
			address:  "127.0.0.1:8080",
			expected: "8080",
		},
		{
			name:     "IPv6 address with port",
			address:  "[::1]:8080",
			expected: "8080",
		},
		{
			name:     "address without port returns empty",
			address:  "localhost",
			expected: "",
		},
		{
			name:     "address with trailing colon returns empty",
			address:  "localhost:",
			expected: "",
		},
		{
			name:     "empty address returns empty",
			address:  "",
			expected: "",
		},
		{
			name:     "high port number",
			address:  "localhost:65535",
			expected: "65535",
		},
		{
			name:     "port only",
			address:  ":8080",
			expected: "8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractFromAddress(tt.address)
			assert.Equal(t, tt.expected, result)
		})
	}
}
