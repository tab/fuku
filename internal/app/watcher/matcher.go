package watcher

import (
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"
)

// Matcher checks if file paths match configured patterns
type Matcher interface {
	Match(path string) bool
}

// matcher implements the Matcher interface
type matcher struct {
	patterns []glob.Glob
	ignores  []glob.Glob
}

// NewMatcher creates a new Matcher from path and ignore patterns
func NewMatcher(paths, ignores []string) (Matcher, error) {
	m := &matcher{
		patterns: make([]glob.Glob, 0, len(paths)),
		ignores:  make([]glob.Glob, 0, len(ignores)),
	}

	for _, p := range paths {
		g, err := glob.Compile(p, '/')
		if err != nil {
			return nil, err
		}

		m.patterns = append(m.patterns, g)
	}

	for _, p := range ignores {
		g, err := glob.Compile(p, '/')
		if err != nil {
			return nil, err
		}

		m.ignores = append(m.ignores, g)
	}

	return m, nil
}

// Match returns true if the path matches any pattern and is not ignored
func (m *matcher) Match(path string) bool {
	path = normalizePath(path)

	for _, ignore := range m.ignores {
		if ignore.Match(path) {
			return false
		}
	}

	for _, pattern := range m.patterns {
		if pattern.Match(path) {
			return true
		}
	}

	return false
}

// normalizePath converts path separators and removes leading ./
func normalizePath(path string) string {
	path = filepath.ToSlash(path)
	path = strings.TrimPrefix(path, "./")

	return path
}
