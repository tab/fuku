package github

import "time"

// Defaults of the release lookup
const (
	DefaultEndpoint = "https://api.github.com/repos/tab/fuku/releases/latest"
	DefaultTimeout  = 3 * time.Second
)

// Options locates the cache of the last fetched tag (an empty CachePath disables the cache)
type Options struct {
	CachePath string
}
