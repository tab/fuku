package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/config"
)

func Test_Resolve(t *testing.T) {
	type result struct {
		tiers []Tier
		error bool
	}

	tests := []struct {
		name     string
		services map[string]*config.Service
		profiles map[string]interface{}
		profile  string
		expected result
	}{
		{
			name:     "Profile not found",
			services: map[string]*config.Service{"api": {Dir: "api"}},
			profiles: map[string]interface{}{},
			profile:  "nonexistent",
			expected: result{tiers: nil, error: true},
		},
		{
			name:     "Service not found in profile",
			services: map[string]*config.Service{"api": {Dir: "api"}},
			profiles: map[string]interface{}{"backend": []interface{}{"api", "nonexistent"}},
			profile:  "backend",
			expected: result{tiers: nil, error: true},
		},
		{
			name:     "Single service with default tier",
			services: map[string]*config.Service{"api": {Dir: "api"}},
			profiles: map[string]interface{}{"api-only": []interface{}{"api"}},
			profile:  "api-only",
			expected: result{tiers: []Tier{{Name: "default", Services: []string{"api"}}}, error: false},
		},
		{
			name: "Multiple services in different tiers",
			services: map[string]*config.Service{
				"storage":      {Dir: "storage", Tier: "foundation"},
				"api":          {Dir: "api", Tier: "platform"},
				"frontend-api": {Dir: "frontend", Tier: "edge"},
			},
			profiles: map[string]interface{}{"all": []interface{}{"storage", "api", "frontend-api"}},
			profile:  "all",
			expected: result{tiers: []Tier{{Name: "foundation", Services: []string{"storage"}}, {Name: "platform", Services: []string{"api"}}, {Name: "edge", Services: []string{"frontend-api"}}}, error: false},
		},
		{
			name: "Multiple services in same tier sorted alphabetically",
			services: map[string]*config.Service{
				"storage":  {Dir: "storage", Tier: "foundation"},
				"database": {Dir: "database", Tier: "foundation"},
				"api":      {Dir: "api", Tier: "platform"},
			},
			profiles: map[string]interface{}{"backend": []interface{}{"storage", "database", "api"}},
			profile:  "backend",
			expected: result{tiers: []Tier{{Name: "foundation", Services: []string{"database", "storage"}}, {Name: "platform", Services: []string{"api"}}}, error: false},
		},
		{
			name: "Wildcard profile returns all services grouped by tier",
			services: map[string]*config.Service{
				"storage": {Dir: "storage", Tier: "foundation"},
				"api":     {Dir: "api", Tier: "platform"},
			},
			profiles: map[string]interface{}{"all": "*"},
			profile:  "all",
			expected: result{tiers: []Tier{{Name: "foundation", Services: []string{"storage"}}, {Name: "platform", Services: []string{"api"}}}, error: false},
		},
		{
			name: "Deduplicates services in profile",
			services: map[string]*config.Service{
				"api": {Dir: "api", Tier: "platform"},
				"web": {Dir: "web", Tier: "edge"},
			},
			profiles: map[string]interface{}{"duplicate": []interface{}{"api", "web", "api"}},
			profile:  "duplicate",
			expected: result{tiers: []Tier{{Name: "platform", Services: []string{"api"}}, {Name: "edge", Services: []string{"web"}}}, error: false},
		},
		{
			name: "Out-of-order services sorted alphabetically within tiers",
			services: map[string]*config.Service{
				"zebra":    {Dir: "zebra", Tier: "platform"},
				"alpha":    {Dir: "alpha", Tier: "platform"},
				"beta":     {Dir: "beta", Tier: "platform"},
				"web":      {Dir: "web", Tier: "edge"},
				"api":      {Dir: "api", Tier: "edge"},
				"frontend": {Dir: "frontend", Tier: "edge"},
			},
			profiles: map[string]interface{}{"mixed": []interface{}{"zebra", "web", "alpha", "api", "beta", "frontend"}},
			profile:  "mixed",
			expected: result{tiers: []Tier{{Name: "platform", Services: []string{"alpha", "beta", "zebra"}}, {Name: "edge", Services: []string{"api", "frontend", "web"}}}, error: false},
		},
		{
			name: "Wildcard profile with multiple services per tier sorted alphabetically",
			services: map[string]*config.Service{
				"user-service": {Dir: "user", Tier: "foundation"},
				"auth-service": {Dir: "auth", Tier: "foundation"},
				"file-storage": {Dir: "file", Tier: "platform"},
				"backend-api":  {Dir: "backend", Tier: "platform"},
				"frontend":     {Dir: "frontend", Tier: "edge"},
			},
			profiles: map[string]interface{}{"all": "*"},
			profile:  "all",
			expected: result{tiers: []Tier{{Name: "foundation", Services: []string{"auth-service", "user-service"}}, {Name: "platform", Services: []string{"backend-api", "file-storage"}}, {Name: "edge", Services: []string{"frontend"}}}, error: false},
		},
		{
			name: "Services with missing tier sorted alphabetically in default tier",
			services: map[string]*config.Service{
				"service-c": {Dir: "c"},
				"service-a": {Dir: "a"},
				"service-b": {Dir: "b"},
			},
			profiles: map[string]interface{}{"default-tier": []interface{}{"service-c", "service-a", "service-b"}},
			profile:  "default-tier",
			expected: result{tiers: []Tier{{Name: "default", Services: []string{"service-a", "service-b", "service-c"}}}, error: false},
		},
		{
			name: "Deduplication with alphabetical sorting",
			services: map[string]*config.Service{
				"zebra": {Dir: "zebra", Tier: "platform"},
				"alpha": {Dir: "alpha", Tier: "platform"},
				"beta":  {Dir: "beta", Tier: "platform"},
			},
			profiles: map[string]interface{}{"dup": []interface{}{"zebra", "alpha", "zebra", "beta", "alpha"}},
			profile:  "dup",
			expected: result{tiers: []Tier{{Name: "platform", Services: []string{"alpha", "beta", "zebra"}}}, error: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Services: tt.services, Profiles: tt.profiles}
			instance := NewDiscovery(cfg)

			tiers, err := instance.Resolve(tt.profile)

			if tt.expected.error {
				assert.Error(t, err)
				assert.Nil(t, tiers)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected.tiers, tiers)
		})
	}
}
