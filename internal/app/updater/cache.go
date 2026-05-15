package updater

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/mod/semver"

	"fuku/internal/config"
)

const (
	cacheTTL      = 24 * time.Hour
	cacheFileName = "version.json"
)

type cache struct {
	Tag       string    `json:"tag"`
	FetchedAt time.Time `json:"fetched_at"`
}

// cachePath returns the absolute path to the cached version file
func cachePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, config.AppName, cacheFileName), nil
}

// readCache reads and decodes the cached version entry; returns ok=false when missing, corrupt, or expired
func readCache(path string) (cache, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return cache{}, false
	}

	var entry cache
	if err := json.Unmarshal(raw, &entry); err != nil {
		return cache{}, false
	}

	if entry.Tag == "" || entry.FetchedAt.IsZero() {
		return cache{}, false
	}

	if !semver.IsValid(normalize(entry.Tag)) {
		return cache{}, false
	}

	if time.Since(entry.FetchedAt) > cacheTTL {
		return cache{}, false
	}

	return entry, true
}

// writeCache serializes the entry to JSON and writes it to path with restrictive permissions
func writeCache(path string, entry cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return os.WriteFile(path, raw, 0o600)
}
