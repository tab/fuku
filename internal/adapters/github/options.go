package github

import "time"

// Defaults of the release lookup
const (
	DefaultEndpoint = "https://api.github.com/repos/tab/fuku/releases/latest"
	DefaultTimeout  = 3 * time.Second
)

// Options locates the release metadata and the cache of the last fetched tag (an empty CachePath disables the cache)
type Options struct {
	Endpoint  string
	Timeout   time.Duration
	CachePath string
}
