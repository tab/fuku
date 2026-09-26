package envfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fuku/internal/model"
)

// Reader reads .env files from service directories
type Reader struct{}

// NewReader creates a reader of .env files
func NewReader() *Reader {
	return &Reader{}
}

// Read returns the entries of the .env file name inside dir in declaration order (a name that leaves dir is an error)
func (r *Reader) Read(dir, name string) ([]model.Env, error) {
	if !isSafeRelativePath(name) {
		return nil, fmt.Errorf("%w: %s", ErrUnsafePath, name)
	}

	path := filepath.Join(dir, name)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open env file: %w", err)
	}

	defer func() { _ = f.Close() }()

	entries, err := parse(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read env file %s: %w", path, err)
	}

	return entries, nil
}

// isSafeRelativePath reports whether name is a relative path that stays inside its parent after cleaning
func isSafeRelativePath(name string) bool {
	if name == "" || filepath.IsAbs(name) {
		return false
	}

	cleaned := filepath.Clean(name)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return false
	}

	return true
}
