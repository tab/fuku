package instance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"fuku/internal/app/errors"
)

// FingerprintLength bounds the project fingerprint exposed to unauthenticated callers
const FingerprintLength = 16

// Identity identifies one running fuku instance and the project directory it serves
type Identity struct {
	ID          string
	Project     string
	Fingerprint string
}

// NewInstance builds the identity of the fuku instance serving the current working directory
func NewInstance() (Identity, error) {
	wd, err := os.Getwd()
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %w", errors.ErrFailedToResolveProject, err)
	}

	project, err := filepath.EvalSymlinks(wd)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %w", errors.ErrFailedToResolveProject, err)
	}

	return Identity{
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
