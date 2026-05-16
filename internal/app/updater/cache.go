package updater

import (
	"encoding/json"
	"fmt"
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
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	return filepath.Join(dir, config.AppName, cacheFileName), nil
}

// readCache decodes the cached version entry (zero cache + nil error means legitimate miss; non-nil error means IO or parse failure)
func readCache(path string) (cache, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cache{}, nil
		}

		return cache{}, fmt.Errorf("read updater cache: %w", err)
	}

	var entry cache
	if err := json.Unmarshal(raw, &entry); err != nil {
		return cache{}, fmt.Errorf("unmarshal updater cache: %w", err)
	}

	if entry.Tag == "" || entry.FetchedAt.IsZero() {
		return cache{}, nil
	}

	if !semver.IsValid(normalize(entry.Tag)) {
		return cache{}, nil
	}

	if time.Since(entry.FetchedAt) > cacheTTL {
		return cache{}, nil
	}

	return entry, nil
}

// writeCache serializes the entry to JSON and writes it to path with restrictive permissions
func writeCache(path string, entry cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create updater cache dir: %w", err)
	}

	raw, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal updater cache: %w", err)
	}

	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write updater cache: %w", err)
	}

	return nil
}
