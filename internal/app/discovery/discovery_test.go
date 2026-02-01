package discovery

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
			name:     "Empty services returns empty tiers",
			services: map[string]*config.Service{},
			profiles: map[string]interface{}{"empty": "*"},
			profile:  "empty",
			expected: result{tiers: []Tier{}, error: false},
		},
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
			name:     "Profile with non-string entry",
			services: map[string]*config.Service{"api": {Dir: "api"}, "web": {Dir: "web"}},
			profiles: map[string]interface{}{"invalid": []interface{}{"api", 123, "web"}},
			profile:  "invalid",
			expected: result{tiers: nil, error: true},
		},
		{
			name:     "Single service with default tier",
			services: map[string]*config.Service{"api": {Dir: "api"}},
			profiles: map[string]interface{}{"api-only": []interface{}{"api"}},
			profile:  "api-only",
			expected: result{
				tiers: []Tier{
					{Name: "default", Services: []string{"api"}},
				},
				error: false,
			},
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
			expected: result{
				tiers: []Tier{
					{Name: "foundation", Services: []string{"storage"}},
					{Name: "platform", Services: []string{"api"}},
					{Name: "edge", Services: []string{"frontend-api"}},
				},
				error: false,
			},
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
			expected: result{
				tiers: []Tier{
					{Name: "foundation", Services: []string{"database", "storage"}},
					{Name: "platform", Services: []string{"api"}},
				},
				error: false,
			},
		},
		{
			name: "Wildcard profile returns all services grouped by tier",
			services: map[string]*config.Service{
				"storage": {Dir: "storage", Tier: "foundation"},
				"api":     {Dir: "api", Tier: "platform"},
			},
			profiles: map[string]interface{}{"all": "*"},
			profile:  "all",
			expected: result{
				tiers: []Tier{
					{Name: "foundation", Services: []string{"storage"}},
					{Name: "platform", Services: []string{"api"}},
				},
				error: false,
			},
		},
		{
			name: "Deduplicates services in profile",
			services: map[string]*config.Service{
				"api": {Dir: "api", Tier: "platform"},
				"web": {Dir: "web", Tier: "edge"},
			},
			profiles: map[string]interface{}{"duplicate": []interface{}{"api", "web", "api"}},
			profile:  "duplicate",
			expected: result{
				tiers: []Tier{
					{Name: "platform", Services: []string{"api"}},
					{Name: "edge", Services: []string{"web"}},
				},
				error: false,
			},
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
			expected: result{
				tiers: []Tier{
					{Name: "platform", Services: []string{"alpha", "beta", "zebra"}},
					{Name: "edge", Services: []string{"api", "frontend", "web"}},
				},
				error: false,
			},
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
			expected: result{
				tiers: []Tier{
					{Name: "foundation", Services: []string{"auth-service", "user-service"}},
					{Name: "platform", Services: []string{"backend-api", "file-storage"}},
					{Name: "edge", Services: []string{"frontend"}},
				},
				error: false,
			},
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
			expected: result{
				tiers: []Tier{
					{Name: "default", Services: []string{"service-a", "service-b", "service-c"}},
				},
				error: false,
			},
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
			expected: result{
				tiers: []Tier{
					{Name: "platform", Services: []string{"alpha", "beta", "zebra"}},
				},
				error: false,
			},
		},
		{
			name: "Custom tier names in YAML order",
			services: map[string]*config.Service{
				"cache":    {Dir: "cache", Tier: "infrastructure"},
				"api":      {Dir: "api", Tier: "backend"},
				"frontend": {Dir: "frontend", Tier: "ui"},
			},
			profiles: map[string]interface{}{"custom": []interface{}{"cache", "api", "frontend"}},
			profile:  "custom",
			expected: result{
				tiers: []Tier{
					{Name: "infrastructure", Services: []string{"cache"}},
					{Name: "backend", Services: []string{"api"}},
					{Name: "ui", Services: []string{"frontend"}},
				},
				error: false,
			},
		},
		{
			name: "Mixed standard and custom tier names",
			services: map[string]*config.Service{
				"db":         {Dir: "db", Tier: "foundation"},
				"middleware": {Dir: "middleware", Tier: "custom-layer"},
				"api":        {Dir: "api", Tier: "platform"},
			},
			profiles: map[string]interface{}{"mixed": []interface{}{"db", "middleware", "api"}},
			profile:  "mixed",
			expected: result{
				tiers: []Tier{
					{Name: "foundation", Services: []string{"db"}},
					{Name: "custom-layer", Services: []string{"middleware"}},
					{Name: "platform", Services: []string{"api"}},
				},
				error: false,
			},
		},
		{
			name: "Backward compatibility with foundation/platform/edge",
			services: map[string]*config.Service{
				"postgres": {Dir: "postgres", Tier: "foundation"},
				"api":      {Dir: "api", Tier: "platform"},
				"web":      {Dir: "web", Tier: "edge"},
			},
			profiles: map[string]interface{}{"classic": []interface{}{"postgres", "api", "web"}},
			profile:  "classic",
			expected: result{
				tiers: []Tier{
					{Name: "foundation", Services: []string{"postgres"}},
					{Name: "platform", Services: []string{"api"}},
					{Name: "edge", Services: []string{"web"}},
				},
				error: false,
			},
		},
		{
			name: "Services inherit defaults.tier in discovery",
			services: map[string]*config.Service{
				"api": {Dir: "api", Tier: "platform"},
				"web": {Dir: "web", Tier: "platform"},
			},
			profiles: map[string]interface{}{"inherited": []interface{}{"api", "web"}},
			profile:  "inherited",
			expected: result{
				tiers: []Tier{
					{Name: "platform", Services: []string{"api", "web"}},
				},
				error: false,
			},
		},
		{
			name: "Unknown tiers coalesce into default tier",
			services: map[string]*config.Service{
				"db":       {Dir: "db", Tier: "foundation"},
				"unknown1": {Dir: "unknown1", Tier: "mystery-tier"},
				"unknown2": {Dir: "unknown2", Tier: "another-unknown"},
			},
			profiles: map[string]interface{}{"with-unknown": []interface{}{"db", "unknown1", "unknown2"}},
			profile:  "with-unknown",
			expected: result{
				tiers: []Tier{
					{Name: "foundation", Services: []string{"db"}},
					{Name: "default", Services: []string{"unknown1", "unknown2"}},
				},
				error: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Services: tt.services,
				Profiles: tt.profiles,
			}
			topology := &config.Topology{
				Order: extractTierOrderFromExpected(tt.expected.tiers),
			}
			instance := NewDiscovery(cfg, topology)

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

func Test_getServicesForProfile(t *testing.T) {
	tests := []struct {
		name            string
		profiles        map[string]interface{}
		profile         string
		expectedError   bool
		expectedService []string
	}{
		{
			name:          "Profile not found",
			profiles:      map[string]interface{}{},
			profile:       "nonexistent",
			expectedError: true,
		},
		{
			name:            "Wildcard profile",
			profiles:        map[string]interface{}{"all": "*"},
			profile:         "all",
			expectedError:   false,
			expectedService: []string{"api", "web"},
		},
		{
			name:            "Single service as string",
			profiles:        map[string]interface{}{"single": "api"},
			profile:         "single",
			expectedError:   false,
			expectedService: []string{"api"},
		},
		{
			name:            "Multiple services as array",
			profiles:        map[string]interface{}{"multi": []interface{}{"api", "web"}},
			profile:         "multi",
			expectedError:   false,
			expectedService: []string{"api", "web"},
		},
		{
			name:          "Non-string entry in profile",
			profiles:      map[string]interface{}{"invalid": []interface{}{"api", 123}},
			profile:       "invalid",
			expectedError: true,
		},
		{
			name:          "Unsupported profile format",
			profiles:      map[string]interface{}{"bad": 123},
			profile:       "bad",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Services: map[string]*config.Service{
					"api": {Dir: "api"},
					"web": {Dir: "web"},
				},
				Profiles: tt.profiles,
			}
			topology := &config.Topology{}
			d := &discovery{cfg: cfg, topology: topology}

			services, err := d.getServicesForProfile(tt.profile)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if tt.profile == "all" {
				assert.ElementsMatch(t, tt.expectedService, services)
			} else {
				assert.Equal(t, tt.expectedService, services)
			}
		})
	}
}

func Test_resolveServiceOrder(t *testing.T) {
	tests := []struct {
		name          string
		services      map[string]*config.Service
		tierOrder     []string
		input         []string
		expectedError bool
		expected      []string
	}{
		{
			name: "Service not found",
			services: map[string]*config.Service{
				"api": {Dir: "api"},
			},
			input:         []string{"api", "nonexistent"},
			expectedError: true,
		},
		{
			name: "Deduplicates services",
			services: map[string]*config.Service{
				"api": {Dir: "api", Tier: "platform"},
				"web": {Dir: "web", Tier: "edge"},
			},
			tierOrder:     []string{"platform", "edge"},
			input:         []string{"api", "web", "api"},
			expectedError: false,
			expected:      []string{"api", "web"},
		},
		{
			name: "Orders by tier",
			services: map[string]*config.Service{
				"web": {Dir: "web", Tier: "edge"},
				"api": {Dir: "api", Tier: "platform"},
				"db":  {Dir: "db", Tier: "foundation"},
			},
			tierOrder:     []string{"foundation", "platform", "edge"},
			input:         []string{"web", "api", "db"},
			expectedError: false,
			expected:      []string{"db", "api", "web"},
		},
		{
			name: "Services with empty tier use default",
			services: map[string]*config.Service{
				"api":   {Dir: "api"},
				"cache": {Dir: "cache", Tier: "foundation"},
			},
			tierOrder:     []string{"foundation", "default"},
			input:         []string{"api", "cache"},
			expectedError: false,
			expected:      []string{"cache", "api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Services: tt.services,
			}
			topology := &config.Topology{
				Order: tt.tierOrder,
			}
			d := &discovery{cfg: cfg, topology: topology}

			result, err := d.resolveServiceOrder(tt.input)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_buildTierIndexMap(t *testing.T) {
	tests := []struct {
		name      string
		tierOrder []string
		expected  map[string]int
	}{
		{
			name:      "Standard tiers",
			tierOrder: []string{"foundation", "platform", "edge"},
			expected: map[string]int{
				"foundation": 0,
				"platform":   1,
				"edge":       2,
				"default":    3,
			},
		},
		{
			name:      "Custom tiers",
			tierOrder: []string{"infrastructure", "backend", "ui"},
			expected: map[string]int{
				"infrastructure": 0,
				"backend":        1,
				"ui":             2,
				"default":        3,
			},
		},
		{
			name:      "Already has default tier",
			tierOrder: []string{"foundation", "default", "edge"},
			expected: map[string]int{
				"foundation": 0,
				"default":    1,
				"edge":       2,
			},
		},
		{
			name:      "Empty tier order",
			tierOrder: []string{},
			expected: map[string]int{
				"default": 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			topology := &config.Topology{Order: tt.tierOrder}
			d := &discovery{cfg: cfg, topology: topology}

			result := d.buildTierIndexMap()

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_getTierIndex(t *testing.T) {
	tests := []struct {
		name         string
		tierIndexMap map[string]int
		tierName     string
		expected     int
	}{
		{
			name: "Known tier",
			tierIndexMap: map[string]int{
				"foundation": 0,
				"platform":   1,
				"edge":       2,
				"default":    3,
			},
			tierName: "platform",
			expected: 1,
		},
		{
			name: "Unknown tier returns default",
			tierIndexMap: map[string]int{
				"foundation": 0,
				"platform":   1,
				"default":    2,
			},
			tierName: "unknown-tier",
			expected: 2,
		},
		{
			name: "Default tier",
			tierIndexMap: map[string]int{
				"foundation": 0,
				"default":    1,
			},
			tierName: "default",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			topology := &config.Topology{}
			d := &discovery{cfg: cfg, topology: topology}

			result := d.getTierIndex(tt.tierName, tt.tierIndexMap)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_groupServicesByTier(t *testing.T) {
	tests := []struct {
		name      string
		services  map[string]*config.Service
		tierOrder []string
		input     []string
		expected  []Tier
	}{
		{
			name: "Groups services by tier",
			services: map[string]*config.Service{
				"db":  {Dir: "db", Tier: "foundation"},
				"api": {Dir: "api", Tier: "platform"},
				"web": {Dir: "web", Tier: "edge"},
			},
			tierOrder: []string{"foundation", "platform", "edge"},
			input:     []string{"db", "api", "web"},
			expected: []Tier{
				{Name: "foundation", Services: []string{"db"}},
				{Name: "platform", Services: []string{"api"}},
				{Name: "edge", Services: []string{"web"}},
			},
		},
		{
			name: "Sorts services alphabetically within tiers",
			services: map[string]*config.Service{
				"zebra": {Dir: "zebra", Tier: "platform"},
				"alpha": {Dir: "alpha", Tier: "platform"},
				"beta":  {Dir: "beta", Tier: "platform"},
			},
			tierOrder: []string{"platform"},
			input:     []string{"zebra", "alpha", "beta"},
			expected: []Tier{
				{Name: "platform", Services: []string{"alpha", "beta", "zebra"}},
			},
		},
		{
			name: "Normalizes empty tier to default",
			services: map[string]*config.Service{
				"api": {Dir: "api"},
				"web": {Dir: "web"},
			},
			tierOrder: []string{"default"},
			input:     []string{"api", "web"},
			expected: []Tier{
				{Name: "default", Services: []string{"api", "web"}},
			},
		},
		{
			name: "Normalizes unknown tier to default",
			services: map[string]*config.Service{
				"db":      {Dir: "db", Tier: "foundation"},
				"unknown": {Dir: "unknown", Tier: "mystery-tier"},
			},
			tierOrder: []string{"foundation", "default"},
			input:     []string{"db", "unknown"},
			expected: []Tier{
				{Name: "foundation", Services: []string{"db"}},
				{Name: "default", Services: []string{"unknown"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Services: tt.services,
			}
			topology := &config.Topology{
				Order: tt.tierOrder,
			}
			d := &discovery{cfg: cfg, topology: topology}

			result := d.groupServicesByTier(tt.input)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func extractTierOrderFromExpected(tiers []Tier) []string {
	tierOrder := make([]string, 0, len(tiers))
	for _, tier := range tiers {
		tierOrder = append(tierOrder, tier.Name)
	}

	return tierOrder
}
