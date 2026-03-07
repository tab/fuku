package sentry

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const (
	configDir       = "fuku"
	telemetryIDFile = "telemetry.id"
)

// loadTelemetryID reads a persistent anonymous telemetry ID from disk, or generates and saves a new one
func loadTelemetryID() string {
	path, err := telemetryIDPath()
	if err != nil {
		return ""
	}

	return loadTelemetryIDFromPath(path)
}

func loadTelemetryIDFromPath(path string) string {
	data, err := os.ReadFile(path)
	if err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id
		}
	}

	id := uuid.NewString()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}

	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		return ""
	}

	return id
}

func telemetryIDPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, configDir, telemetryIDFile), nil
}
