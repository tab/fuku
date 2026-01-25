package watcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewMatcher(t *testing.T) {
	tests := []struct {
		name      string
		paths     []string
		ignores   []string
		expectErr bool
	}{
		{
			name:      "valid patterns",
			paths:     []string{"**/*.go", "*.yaml"},
			ignores:   []string{"*_test.go"},
			expectErr: false,
		},
		{
			name:      "empty patterns",
			paths:     []string{},
			ignores:   []string{},
			expectErr: false,
		},
		{
			name:      "invalid path pattern",
			paths:     []string{"[invalid"},
			ignores:   []string{},
			expectErr: true,
		},
		{
			name:      "invalid ignore pattern",
			paths:     []string{"*.go"},
			ignores:   []string{"[invalid"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.paths, tt.ignores)

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
		name    string
		paths   []string
		ignores []string
		file    string
		expect  bool
	}{
		{
			name:    "matches go file",
			paths:   []string{"**/*.go"},
			ignores: []string{},
			file:    "src/main.go",
			expect:  true,
		},
		{
			name:    "matches root go file with star pattern",
			paths:   []string{"*.go", "**/*.go"},
			ignores: []string{},
			file:    "main.go",
			expect:  true,
		},
		{
			name:    "ignores test file",
			paths:   []string{"**/*.go"},
			ignores: []string{"*_test.go"},
			file:    "main_test.go",
			expect:  false,
		},
		{
			name:    "ignores nested test file",
			paths:   []string{"**/*.go"},
			ignores: []string{"**/*_test.go"},
			file:    "pkg/handler_test.go",
			expect:  false,
		},
		{
			name:    "ignores vendor directory",
			paths:   []string{"**/*.go"},
			ignores: []string{"vendor/**"},
			file:    "vendor/lib/file.go",
			expect:  false,
		},
		{
			name:    "no match for yaml when watching go",
			paths:   []string{"**/*.go"},
			ignores: []string{},
			file:    "config.yaml",
			expect:  false,
		},
		{
			name:    "matches multiple patterns",
			paths:   []string{"*.go", "**/*.go", "*.yaml", "**/*.yaml"},
			ignores: []string{},
			file:    "config.yaml",
			expect:  true,
		},
		{
			name:    "handles leading dot-slash",
			paths:   []string{"**/*.go"},
			ignores: []string{},
			file:    "./src/main.go",
			expect:  true,
		},
		{
			name:    "matches mock file",
			paths:   []string{"**/*.go"},
			ignores: []string{"mock_*.go", "**/*_mock.go"},
			file:    "mock_service.go",
			expect:  false,
		},
		{
			name:    "matches nested mock file",
			paths:   []string{"**/*.go"},
			ignores: []string{"mock_*.go", "**/*_mock.go"},
			file:    "internal/service_mock.go",
			expect:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.paths, tt.ignores)
			assert.NoError(t, err)

			result := m.Match(tt.file)
			assert.Equal(t, tt.expect, result)
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
