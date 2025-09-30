package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/config"
)

func Test_resolveServiceOrder(t *testing.T) {
	tests := []struct {
		name     string
		services map[string]*config.Service
		input    []string
		expected []string
		wantErr  bool
	}{
		{
			name: "simple dependency chain",
			services: map[string]*config.Service{
				"a": {Dir: "a"},
				"b": {Dir: "b", DependsOn: []string{"a"}},
				"c": {Dir: "c", DependsOn: []string{"b"}},
			},
			input:    []string{"c"},
			expected: []string{"a", "b", "c"},
			wantErr:  false,
		},
		{
			name: "circular dependency",
			services: map[string]*config.Service{
				"a": {Dir: "a", DependsOn: []string{"b"}},
				"b": {Dir: "b", DependsOn: []string{"a"}},
			},
			input:    []string{"a"},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "missing service",
			services: map[string]*config.Service{
				"a": {Dir: "a"},
			},
			input:    []string{"nonexistent"},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Services: tt.services}

			result, err := resolveServiceOrder(cfg, tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func Test_groupByDependencyLevel(t *testing.T) {
	tests := []struct {
		name     string
		services map[string]*config.Service
		input    []string
		expected [][]string
	}{
		{
			name: "parallel services - no dependencies",
			services: map[string]*config.Service{
				"api":          {Dir: "api"},
				"frontend-api": {Dir: "frontend-api"},
			},
			input:    []string{"api", "frontend-api"},
			expected: [][]string{{"api", "frontend-api"}},
		},
		{
			name: "two batches - same level dependencies",
			services: map[string]*config.Service{
				"api":          {Dir: "api"},
				"frontend-api": {Dir: "frontend-api"},
				"user":         {Dir: "user", DependsOn: []string{"api"}},
				"storage":      {Dir: "storage", DependsOn: []string{"api"}},
			},
			input:    []string{"api", "frontend-api", "user", "storage"},
			expected: [][]string{{"api", "frontend-api"}, {"user", "storage"}},
		},
		{
			name: "three batches - sequential dependencies",
			services: map[string]*config.Service{
				"api":          {Dir: "api"},
				"frontend-api": {Dir: "frontend-api"},
				"user":         {Dir: "user", DependsOn: []string{"api"}},
				"storage":      {Dir: "storage", DependsOn: []string{"user"}},
			},
			input:    []string{"api", "frontend-api", "user", "storage"},
			expected: [][]string{{"api", "frontend-api"}, {"user"}, {"storage"}},
		},
		{
			name: "complex dependencies",
			services: map[string]*config.Service{
				"a": {Dir: "a"},
				"b": {Dir: "b", DependsOn: []string{"a"}},
				"c": {Dir: "c", DependsOn: []string{"a"}},
				"d": {Dir: "d", DependsOn: []string{"b", "c"}},
			},
			input:    []string{"a", "b", "c", "d"},
			expected: [][]string{{"a"}, {"b", "c"}, {"d"}},
		},
		{
			name: "single service",
			services: map[string]*config.Service{
				"api": {Dir: "api"},
			},
			input:    []string{"api"},
			expected: [][]string{{"api"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Services: tt.services}

			result := groupByDependencyLevel(tt.input, cfg)

			assert.Equal(t, len(tt.expected), len(result))
			for i := range tt.expected {
				assert.ElementsMatch(t, tt.expected[i], result[i])
			}
		})
	}
}
