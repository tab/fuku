package watcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewMatcher(t *testing.T) {
	tests := []struct {
		name      string
		includes  []string
		ignores   []string
		expectErr bool
	}{
		{
			name:      "valid patterns",
			includes:  []string{"**/*.go", "*.yaml"},
			ignores:   []string{"*_test.go"},
			expectErr: false,
		},
		{
			name:      "empty patterns",
			includes:  []string{},
			ignores:   []string{},
			expectErr: false,
		},
		{
			name:      "invalid include pattern",
			includes:  []string{"[invalid"},
			ignores:   []string{},
			expectErr: true,
		},
		{
			name:      "invalid ignore pattern",
			includes:  []string{"*.go"},
			ignores:   []string{"[invalid"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.includes, tt.ignores)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, m)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, m)
			}
		})
	}
}

func Test_Matcher_Match(t *testing.T) {
	tests := []struct {
		name     string
		includes []string
		ignores  []string
		file     string
		expect   bool
	}{
		{
			name:     "matches go file",
			includes: []string{"**/*.go"},
			ignores:  []string{},
			file:     "src/main.go",
			expect:   true,
		},
		{
			name:     "matches root go file with double star pattern",
			includes: []string{"**/*.go"},
			ignores:  []string{},
			file:     "main.go",
			expect:   true,
		},
		{
			name:     "matches root go file with star pattern",
			includes: []string{"*.go", "**/*.go"},
			ignores:  []string{},
			file:     "main.go",
			expect:   true,
		},
		{
			name:     "ignores test file",
			includes: []string{"**/*.go"},
			ignores:  []string{"*_test.go"},
			file:     "main_test.go",
			expect:   false,
		},
		{
			name:     "ignores nested test file",
			includes: []string{"**/*.go"},
			ignores:  []string{"**/*_test.go"},
			file:     "pkg/handler_test.go",
			expect:   false,
		},
		{
			name:     "ignores vendor directory",
			includes: []string{"**/*.go"},
			ignores:  []string{"vendor/**"},
			file:     "vendor/lib/file.go",
			expect:   false,
		},
		{
			name:     "no match for yaml when watching go",
			includes: []string{"**/*.go"},
			ignores:  []string{},
			file:     "config.yaml",
			expect:   false,
		},
		{
			name:     "matches multiple patterns",
			includes: []string{"*.go", "**/*.go", "*.yaml", "**/*.yaml"},
			ignores:  []string{},
			file:     "config.yaml",
			expect:   true,
		},
		{
			name:     "handles leading dot-slash",
			includes: []string{"**/*.go"},
			ignores:  []string{},
			file:     "./src/main.go",
			expect:   true,
		},
		{
			name:     "matches mock file",
			includes: []string{"**/*.go"},
			ignores:  []string{"mock_*.go", "**/*_mock.go"},
			file:     "mock_service.go",
			expect:   false,
		},
		{
			name:     "matches nested mock file",
			includes: []string{"**/*.go"},
			ignores:  []string{"mock_*.go", "**/*_mock.go"},
			file:     "internal/service_mock.go",
			expect:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.includes, tt.ignores)
			assert.NoError(t, err)

			result := m.Match(tt.file)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func Test_Matcher_MatchDir(t *testing.T) {
	tests := []struct {
		name    string
		ignores []string
		dir     string
		expect  bool
	}{
		{
			name:    "skips .git directory",
			ignores: []string{".git/**"},
			dir:     ".git",
			expect:  true,
		},
		{
			name:    "skips vendor directory",
			ignores: []string{"vendor/**"},
			dir:     "vendor",
			expect:  true,
		},
		{
			name:    "skips nested ignored directory",
			ignores: []string{"**/node_modules/**"},
			dir:     "frontend/node_modules",
			expect:  true,
		},
		{
			name:    "does not skip src directory",
			ignores: []string{".git/**", "vendor/**"},
			dir:     "src",
			expect:  false,
		},
		{
			name:    "does not skip with file-only patterns",
			ignores: []string{"*_test.go"},
			dir:     "pkg",
			expect:  false,
		},
		{
			name:    "no ignores skips nothing",
			ignores: []string{},
			dir:     ".git",
			expect:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher([]string{"**/*.go"}, tt.ignores)
			assert.NoError(t, err)

			result := m.MatchDir(tt.dir)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func Test_expandPatterns(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		expected []string
	}{
		{
			name:     "expands double star prefix",
			patterns: []string{"**/*.go"},
			expected: []string{"**/*.go", "*.go"},
		},
		{
			name:     "expands multiple patterns",
			patterns: []string{"**/*.go", "**/*.yaml"},
			expected: []string{"**/*.go", "*.go", "**/*.yaml", "*.yaml"},
		},
		{
			name:     "keeps patterns without double star prefix",
			patterns: []string{"*.go", "vendor/**"},
			expected: []string{"*.go", "vendor/**"},
		},
		{
			name:     "handles mixed patterns",
			patterns: []string{"**/*.go", "*.yaml", "**/test/**"},
			expected: []string{"**/*.go", "*.go", "*.yaml", "**/test/**", "test/**"},
		},
		{
			name:     "handles empty patterns",
			patterns: []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPatterns(tt.patterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_normalizePath(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "removes leading dot-slash",
			input:  "./src/main.go",
			expect: "src/main.go",
		},
		{
			name:   "keeps path without prefix",
			input:  "src/main.go",
			expect: "src/main.go",
		},
		{
			name:   "handles root file",
			input:  "main.go",
			expect: "main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}
