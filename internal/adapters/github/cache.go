package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"fuku/internal/adapters/instance"
)

const (
	cacheTTL      = 24 * time.Hour
	cacheFileName = "version.json"
)

// cache is the on-disk record of the last fetched release tag
type cache struct {
	Tag       string    `json:"tag"`
	FetchedAt time.Time `json:"fetched_at"`
}

// DefaultCachePath returns the cache file in the user's config directory, or empty when that directory is unavailable
func DefaultCachePath() string {
	path, err := instance.UserConfigPath(cacheFileName)
	if err != nil {
		return ""
	}

	return path
}

// readCache decodes the cached release entry (a zero entry with a nil error is a miss)
func readCache(path string) (cache, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cache{}, nil
	}

	if err != nil {
		return cache{}, fmt.Errorf("read updater cache: %w", err)
	}

	var entry cache
	if err := json.Unmarshal(raw, &entry); err != nil {
		return cache{}, fmt.Errorf("unmarshal updater cache: %w", err)
	}

	if entry.Tag == "" || entry.FetchedAt.IsZero() {
		return cache{}, nil
	}

	if time.Since(entry.FetchedAt) > cacheTTL {
		return cache{}, nil
	}

	return entry, nil
}

// writeCache serializes the entry to JSON and writes it to path with restrictive permissions
func writeCache(path string, entry cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create updater cache dir: %w", err)
	}

	raw, _ := json.Marshal(entry)

	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write updater cache: %w", err)
	}

	return nil
}
