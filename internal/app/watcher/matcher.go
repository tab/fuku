package watcher

import (
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"
)

// Matcher checks if file paths match configured patterns
type Matcher interface {
	Match(path string) bool
	MatchDir(dirPath string) bool
}

// matcher implements the Matcher interface
type matcher struct {
	patterns []glob.Glob
	ignores  []glob.Glob
}

// NewMatcher creates a new Matcher from include and ignore patterns
func NewMatcher(includes, ignores []string) (Matcher, error) {
	m := &matcher{
		patterns: make([]glob.Glob, 0, len(includes)),
		ignores:  make([]glob.Glob, 0, len(ignores)),
	}

	for _, p := range expandPatterns(includes) {
		g, err := glob.Compile(p, '/')
		if err != nil {
			return nil, err
		}

		m.patterns = append(m.patterns, g)
	}

	for _, p := range expandPatterns(ignores) {
		g, err := glob.Compile(p, '/')
		if err != nil {
			return nil, err
		}

		m.ignores = append(m.ignores, g)
	}

	return m, nil
}

// expandPatterns expands patterns starting with **/ to also match at root level
func expandPatterns(patterns []string) []string {
	expanded := make([]string, 0, len(patterns)*2)

	for _, p := range patterns {
		expanded = append(expanded, p)

		if strings.HasPrefix(p, "**/") {
			rootVariant := strings.TrimPrefix(p, "**/")
			expanded = append(expanded, rootVariant)
		}
	}

	return expanded
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

// MatchDir returns true if a directory should be skipped based on ignore patterns
func (m *matcher) MatchDir(dirPath string) bool {
	probe := normalizePath(dirPath + "/_probe")

	for _, ignore := range m.ignores {
		if ignore.Match(probe) {
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
