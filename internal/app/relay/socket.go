package relay

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fuku/internal/app/errors"
	"fuku/internal/config"
)

// SocketPathForProfile constructs the socket path for a given profile
func SocketPathForProfile(socketDir, profile string) string {
	return filepath.Join(socketDir, fmt.Sprintf("%s%s%s", config.SocketPrefix, profile, config.SocketSuffix))
}

// FindSocket finds the socket for a running fuku instance in the given directory
func FindSocket(socketDir, profile string) (string, error) {
	if profile != "" {
		socketPath := SocketPathForProfile(socketDir, profile)
		if _, err := os.Stat(socketPath); err == nil {
			return socketPath, nil
		}

		return "", fmt.Errorf("%w: '%s'", errors.ErrInstanceNotFound, profile)
	}

	pattern := SocketPathForProfile(socketDir, "*")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errors.ErrSocketSearchFailed, err)
	}

	if len(matches) == 0 {
		return "", errors.ErrNoInstanceRunning
	}

	if len(matches) > 1 {
		profiles := make([]string, len(matches))
		for i, m := range matches {
			base := filepath.Base(m)
			profiles[i] = strings.TrimSuffix(strings.TrimPrefix(base, config.SocketPrefix), config.SocketSuffix)
		}

		return "", fmt.Errorf("%w, use: fuku logs --profile <name>, available: %v", errors.ErrMultipleInstancesRunning, profiles)
	}

	return matches[0], nil
}

// Cleanup removes all stale fuku socket files from the given directory
func Cleanup(socketDir string) error {
	pattern := SocketPathForProfile(socketDir, "*")

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

// Identify reads the status banner a running instance sends, without following its log stream
// (the server answers a subscribe with the banner, so the shortest possible subscription is used)
func Identify(ctx context.Context, socketPath string) (StatusMessage, error) {
	var dialer net.Dialer

	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return StatusMessage{}, fmt.Errorf("%w: %w", errors.ErrFailedToConnectSocket, err)
	}

	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(config.SocketDialTimeout)); err != nil {
		return StatusMessage{}, fmt.Errorf("%w: %w", errors.ErrFailedToReadSocket, err)
	}

	request, err := json.Marshal(SubscribeRequest{Type: MessageSubscribe, Tail: 1, NoFollow: true})
	if err != nil {
		return StatusMessage{}, fmt.Errorf("%w: %w", errors.ErrFailedToMarshalMessage, err)
	}

	if _, err := conn.Write(append(request, '\n')); err != nil {
		return StatusMessage{}, fmt.Errorf("%w: %w", errors.ErrFailedToWriteSocket, err)
	}

	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return StatusMessage{}, fmt.Errorf("%w: %w", errors.ErrFailedToReadSocket, err)
	}

	var status StatusMessage

	if err := json.Unmarshal(line, &status); err != nil {
		return StatusMessage{}, fmt.Errorf("%w: %w", errors.ErrFailedToReadSocket, err)
	}

	return status, nil
}
