package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NormalizeQuery(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "lowercase passthrough",
			input: "api",
			want:  "api",
		},
		{
			name:  "uppercase to lowercase",
			input: "API",
			want:  "api",
		},
		{
			name:  "mixed case",
			input: "ApI-Server",
			want:  "api-server",
		},
		{
			name:  "trim leading dash",
			input: "-api",
			want:  "api",
		},
		{
			name:  "trim trailing dash",
			input: "api-",
			want:  "api",
		},
		{
			name:  "trim leading underscore",
			input: "_api",
			want:  "api",
		},
		{
			name:  "trim trailing underscore",
			input: "api_",
			want:  "api",
		},
		{
			name:  "trim leading spaces",
			input: "  api",
			want:  "api",
		},
		{
			name:  "trim trailing spaces",
			input: "api  ",
			want:  "api",
		},
		{
			name:  "trim mixed separators",
			input: "- _api_ -",
			want:  "api",
		},
		{
			name:  "internal separators preserved",
			input: "api-server_v2",
			want:  "api-server_v2",
		},
		{
			name:  "only separators",
			input: "---",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeQuery(tt.input))
		})
	}
}

func Test_FilterServiceIDs(t *testing.T) {
	services := map[string]*ServiceState{
		"id-api":    {Name: "api-server"},
		"id-web":    {Name: "web-app"},
		"id-db":     {Name: "database"},
		"id-cache":  {Name: "cache-server"},
		"id-worker": {Name: "worker"},
	}
	allIDs := []string{"id-api", "id-web", "id-db", "id-cache", "id-worker"}

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "empty query returns all",
			query: "",
			want:  allIDs,
		},
		{
			name:  "match single service",
			query: "web",
			want:  []string{"id-web"},
		},
		{
			name:  "match multiple services",
			query: "server",
			want:  []string{"id-api", "id-cache"},
		},
		{
			name:  "case insensitive",
			query: "DATABASE",
			want:  []string{"id-db"},
		},
		{
			name:  "no matches returns empty",
			query: "nosuchservice",
			want:  []string{},
		},
		{
			name:  "preserves order",
			query: "er",
			want:  []string{"id-api", "id-cache", "id-worker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterServiceIDs(tt.query, allIDs, services)
			if len(tt.want) == 0 {
				assert.Empty(t, result)
			} else {
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func Test_FilterTiers(t *testing.T) {
	services := map[string]*ServiceState{
		"id-db":     {Name: "database"},
		"id-cache":  {Name: "cache-server"},
		"id-api":    {Name: "api-server"},
		"id-web":    {Name: "web-app"},
		"id-worker": {Name: "worker"},
	}
	tiers := []Tier{
		{
			Name:     "foundation",
			Services: []string{"id-db", "id-cache"},
			Ready:    true,
		},
		{
			Name:     "application",
			Services: []string{"id-api", "id-web"},
			Ready:    false,
		},
		{
			Name:     "background",
			Services: []string{"id-worker"},
			Ready:    true,
		},
	}

	tests := []struct {
		name      string
		query     string
		wantTiers int
		assertFn  func(t *testing.T, result []Tier)
	}{
		{
			name:      "empty query returns all tiers",
			query:     "",
			wantTiers: 3,
			assertFn: func(t *testing.T, result []Tier) {
				assert.Equal(t, tiers, result)
			},
		},
		{
			name:      "match across tiers",
			query:     "server",
			wantTiers: 2,
			assertFn: func(t *testing.T, result []Tier) {
				assert.Equal(t, "foundation", result[0].Name)
				assert.Equal(t, []string{"id-cache"}, result[0].Services)
				assert.True(t, result[0].Ready)
				assert.Equal(t, "application", result[1].Name)
				assert.Equal(t, []string{"id-api"}, result[1].Services)
				assert.False(t, result[1].Ready)
			},
		},
		{
			name:      "empty tier omitted",
			query:     "web",
			wantTiers: 1,
			assertFn: func(t *testing.T, result []Tier) {
				assert.Equal(t, "application", result[0].Name)
				assert.Equal(t, []string{"id-web"}, result[0].Services)
			},
		},
		{
			name:      "no matches returns empty",
			query:     "nosuchservice",
			wantTiers: 0,
			assertFn: func(t *testing.T, result []Tier) {
				assert.Empty(t, result)
			},
		},
		{
			name:      "single tier match",
			query:     "worker",
			wantTiers: 1,
			assertFn: func(t *testing.T, result []Tier) {
				assert.Equal(t, "background", result[0].Name)
				assert.Equal(t, []string{"id-worker"}, result[0].Services)
				assert.True(t, result[0].Ready)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterTiers(tt.query, tiers, services)
			assert.Len(t, result, tt.wantTiers)
			tt.assertFn(t, result)
		})
	}
}
