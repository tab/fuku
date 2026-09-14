package relay

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"fuku/internal/app/errors"
	"fuku/internal/app/instance"
	"fuku/internal/config"
)

// FindSocket returns the socket of the running fuku instance for the project with the given fingerprint
func FindSocket(socketDir, fingerprint string) (string, error) {
	socketPath := instance.SocketPath(socketDir, fingerprint)

	info, err := os.Lstat(socketPath)
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return "", errors.ErrNoInstanceRunning
	}

	return socketPath, nil
}

// Cleanup removes all stale fuku socket files from the given directory
func Cleanup(socketDir string) error {
	pattern := instance.SocketPath(socketDir, "*")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob for stale sockets: %w", err)
	}

	var failed []string

	for _, socketPath := range matches {
		info, err := os.Lstat(socketPath)
		if err != nil || info.Mode()&os.ModeSocket == 0 {
			continue
		}

		conn, err := net.DialTimeout("unix", socketPath, config.SocketDialTimeout)
		if err == nil {
			conn.Close()
			continue
		}

		if err := os.Remove(socketPath); err != nil {
			failed = append(failed, filepath.Base(socketPath))
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("%w: %v", errors.ErrFailedToCleanupSocket, failed)
	}

	return nil
}
