package logsocket

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"fuku/internal/adapters/instance"
	"fuku/internal/contracts"
)

// findSocket returns the socket of the running fuku instance for the project with the given fingerprint
func findSocket(socketDir, fingerprint string) (string, error) {
	socketPath := instance.SocketPath(socketDir, fingerprint)

	info, err := os.Lstat(socketPath)
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return "", contracts.ErrNoInstanceRunning
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return "", contracts.ErrNoInstanceRunning
	}

	return socketPath, nil
}

// cleanup removes the stale fuku socket files from the given directory but the project's own, which start replaces
func cleanup(socketDir, fingerprint string) error {
	own := instance.SocketPath(socketDir, fingerprint)
	pattern := instance.SocketPath(socketDir, "*")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob for stale sockets: %w", err)
	}

	var failed []string

	for _, socketPath := range matches {
		if socketPath == own {
			continue
		}

		info, err := os.Lstat(socketPath)
		if err != nil || info.Mode()&os.ModeSocket == 0 {
			continue
		}

		if instance.ProbeSocket(socketPath) == nil {
			continue
		}

		if err := os.Remove(socketPath); err != nil {
			failed = append(failed, filepath.Base(socketPath))
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to cleanup stale socket: %v", failed)
	}

	return nil
}
