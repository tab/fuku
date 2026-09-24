package instance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"fuku/internal/model"
)

// FingerprintLength bounds the project fingerprint exposed to unauthenticated callers
const FingerprintLength = 16

// Socket location and the dial timeout of a liveness probe against it
const (
	SocketDir         = "/tmp"
	SocketDialTimeout = 100 * time.Millisecond

	socketPrefix = "fuku-"
	socketSuffix = ".sock"
	lockSuffix   = ".lock"
)

// NewInstance builds the identity of the fuku instance serving the current working directory
func NewInstance() (model.Instance, error) {
	wd, err := os.Getwd()
	if err != nil {
		return model.Instance{}, fmt.Errorf("%w: %w", ErrFailedToResolveProject, err)
	}

	project, err := filepath.EvalSymlinks(wd)
	if err != nil {
		return model.Instance{}, fmt.Errorf("%w: %w", ErrFailedToResolveProject, err)
	}

	return model.Instance{
		ID:          uuid.NewString(),
		Project:     project,
		Fingerprint: Fingerprint(project),
	}, nil
}

// Fingerprint reduces a project directory to a stable identifier that does not disclose the path
func Fingerprint(project string) string {
	sum := sha256.Sum256([]byte(project))

	return hex.EncodeToString(sum[:])[:FingerprintLength]
}

// SocketPath returns the relay socket of the project with the given fingerprint inside socketDir
func SocketPath(socketDir, fingerprint string) string {
	return filepath.Join(socketDir, socketPrefix+fingerprint+socketSuffix)
}

// lockPath returns the lock file of the project with the given fingerprint
func lockPath(fingerprint string) string {
	return filepath.Join(SocketDir, socketPrefix+fingerprint+lockSuffix)
}
