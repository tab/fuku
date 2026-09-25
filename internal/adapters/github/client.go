package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

// HTTPDoer sends an HTTP request
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Logger is the logging surface the release client writes through
type Logger interface {
	Debug(msg string, args ...any)
}

// Client fetches the latest fuku release from the GitHub releases API through a daily cache
type Client struct {
	options Options
	doer    HTTPDoer
	log     Logger
}

// NewClient creates a release client that sends its requests through doer
func NewClient(options Options, doer HTTPDoer, log Logger) *Client {
	return &Client{
		options: options,
		doer:    doer,
		log:     log,
	}
}

// newHTTPClient creates the HTTP client the release lookup sends through
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: DefaultTimeout}
}

// Latest returns the cached release while it is fresh, otherwise fetches it from GitHub and caches the tag
func (c *Client) Latest(ctx context.Context) (model.Release, error) {
	if c.options.CachePath == "" {
		c.log.Debug("Cache path unavailable, proceeding without cache")
	}

	if tag, ok := c.cached(); ok {
		return model.Release{Tag: tag}, nil
	}

	tag, err := c.fetch(ctx)
	if err != nil {
		return model.Release{}, err
	}

	c.persist(tag)

	return model.Release{Tag: tag}, nil
}

// cached returns the cached tag when the cache is enabled, readable and fresh
func (c *Client) cached() (string, bool) {
	if c.options.CachePath == "" {
		return "", false
	}

	entry, err := readCache(c.options.CachePath)
	if err != nil {
		c.log.Debug("Read cache failed", "error", err)

		return "", false
	}

	return entry.Tag, entry.Tag != ""
}

// persist records the fetched tag in the cache when the cache is enabled
func (c *Client) persist(tag string) {
	if c.options.CachePath == "" {
		return
	}

	if err := writeCache(c.options.CachePath, cache{Tag: tag, FetchedAt: time.Now()}); err != nil {
		c.log.Debug("Write cache failed", "error", err)
	}
}

// releaseResponse is the part of the GitHub release payload the client reads
type releaseResponse struct {
	TagName string `json:"tag_name"`
}

// fetch requests the latest release from the endpoint and returns its tag
func (c *Client) fetch(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DefaultEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("build GitHub release request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "fuku/"+buildinfo.Version)

	resp, err := c.doer.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch GitHub release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%w: %d", ErrUnexpectedReleaseStatus, resp.StatusCode)
	}

	var body releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode GitHub release response: %w", err)
	}

	if body.TagName == "" {
		return "", ErrEmptyReleaseTag
	}

	return body.TagName, nil
}
