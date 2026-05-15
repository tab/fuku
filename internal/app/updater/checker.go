package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/mod/semver"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

const (
	defaultReleaseURL = "https://api.github.com/repos/tab/fuku/releases/latest"
	httpTimeout       = 3 * time.Second
)

var (
	releaseURL  = defaultReleaseURL
	cachePathFn = cachePath
)

// Checker performs a one-shot version check and publishes EventUpdateAvailable when a newer release exists
type Checker interface {
	Run(ctx context.Context)
}

type checker struct {
	cfg        *config.Config
	bus        bus.Bus
	httpClient *http.Client
	log        logger.Logger
}

// NewChecker creates a new version checker
func NewChecker(cfg *config.Config, b bus.Bus, log logger.Logger) Checker {
	return &checker{
		bus:        b,
		cfg:        cfg,
		log:        log,
		httpClient: &http.Client{Timeout: httpTimeout},
	}
}

// Run consults the cache and GitHub releases API; publishes EventUpdateAvailable when a newer version exists
func (c *checker) Run(ctx context.Context) {
	if c.cfg.UpdaterDisabled() {
		return
	}

	path, pathErr := cachePathFn()
	if pathErr != nil {
		c.log.Debug().Err(pathErr).Msg("updater: cache path unavailable; proceeding without cache")
	}

	cacheAvailable := pathErr == nil

	if cacheAvailable && c.useCachedTag(path) {
		return
	}

	tag, err := c.fetchLatest(ctx)
	if err != nil {
		c.log.Debug().Err(err).Msg("updater: fetch latest release failed")
		return
	}

	if !semver.IsValid(normalize(tag)) {
		c.log.Debug().Str("tag", tag).Msg("updater: skip cache for invalid semver tag")
		return
	}

	if cacheAvailable {
		c.persistTag(path, tag)
	}

	c.evaluate(tag)
}

func (c *checker) useCachedTag(path string) bool {
	entry, ok := readCache(path)
	if !ok {
		return false
	}

	c.evaluate(entry.Tag)

	return true
}

func (c *checker) persistTag(path, tag string) {
	err := writeCache(path, cache{Tag: tag, FetchedAt: time.Now()})
	if err != nil {
		c.log.Debug().Err(err).Msg("updater: write cache failed")
	}
}

func (c *checker) evaluate(latest string) {
	if !isNewer(config.Version, latest) {
		return
	}

	c.bus.Publish(bus.Message{
		Type: bus.EventUpdateAvailable,
		Data: bus.UpdateAvailable{Version: normalize(latest)},
	})
}

type releaseResponse struct {
	TagName string `json:"tag_name"`
}

func (c *checker) fetchLatest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "fuku/"+config.Version)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%w: %d", errors.ErrUnexpectedReleaseStatus, resp.StatusCode)
	}

	var body releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}

	if body.TagName == "" {
		return "", errors.ErrEmptyReleaseTag
	}

	return body.TagName, nil
}
