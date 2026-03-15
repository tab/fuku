package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_DefaultTopology(t *testing.T) {
	topology := DefaultTopology()

	assert.NotNil(t, topology.TierServices)
	assert.Empty(t, topology.Order)
	assert.True(t, topology.HasDefaultOnly)
}

func Test_ParseTierOrder(t *testing.T) {
	tests := []struct {
		name              string
		yaml              string
		expectedTierOrder []string
		expectedServices  map[string][]string
	}{
		{
			name: "standard tiers in order",
			yaml: `services:
  storage:
    tier: foundation
  api:
    tier: platform
  web:
    tier: edge`,
			expectedTierOrder: []string{"foundation", "platform", "edge"},
			expectedServices:  map[string][]string{"foundation": {"storage"}, "platform": {"api"}, "edge": {"web"}},
		},
		{
			name: "custom tier names",
			yaml: `services:
  db:
    tier: infrastructure
  svc:
    tier: middleware
  ui:
    tier: frontend`,
			expectedTierOrder: []string{"infrastructure", "middleware", "frontend"},
			expectedServices:  map[string][]string{"infrastructure": {"db"}, "middleware": {"svc"}, "frontend": {"ui"}},
		},
		{
			name: "mixed tiers with duplicates",
			yaml: `services:
  db1:
    tier: foundation
  api1:
    tier: platform
  db2:
    tier: foundation
  api2:
    tier: platform`,
			expectedTierOrder: []string{"foundation", "platform"},
			expectedServices:  map[string][]string{"foundation": {"db1", "db2"}, "platform": {"api1", "api2"}},
		},
		{
			name: "services without tiers",
			yaml: `services:
  svc1:
    dir: ./svc1
  svc2:
    dir: ./svc2`,
			expectedTierOrder: []string{"default"},
			expectedServices:  map[string][]string{"default": {"svc1", "svc2"}},
		},
		{
			name: "mixed with and without tiers",
			yaml: `services:
  db:
    tier: foundation
  svc1:
    dir: ./svc1
  api:
    tier: platform
  svc2:
    dir: ./svc2`,
			expectedTierOrder: []string{"foundation", "platform", "default"},
			expectedServices:  map[string][]string{"foundation": {"db"}, "default": {"svc1", "svc2"}, "platform": {"api"}},
		},
		{
			name: "case insensitive and whitespace trimming",
			yaml: `services:
  svc1:
    tier: " Foundation "
  svc2:
    tier: PLATFORM
  svc3:
    tier: foundation`,
			expectedTierOrder: []string{"foundation", "platform"},
			expectedServices:  map[string][]string{"foundation": {"svc1", "svc3"}, "platform": {"svc2"}},
		},
		{
			name: "tier inherited via merge key",
			yaml: `x-svc: &svc
  tier: foundation
services:
  api:
    <<: *svc
    dir: services/api`,
			expectedTierOrder: []string{"foundation"},
			expectedServices:  map[string][]string{"foundation": {"api"}},
		},
		{
			name: "tier inherited via layered merge keys",
			yaml: `x-base: &base
  tier: foundation
x-svc: &svc
  <<: *base
  readiness:
    type: http
services:
  api:
    <<: *svc
    dir: services/api`,
			expectedTierOrder: []string{"foundation"},
			expectedServices:  map[string][]string{"foundation": {"api"}},
		},
		{
			name:              "empty services",
			yaml:              `services: {}`,
			expectedTierOrder: []string{},
			expectedServices:  map[string][]string{},
		},
		{
			name: "services inherit defaults.tier",
			yaml: `services:
  api:
    dir: ./api
  web:
    dir: ./web
defaults:
  tier: platform`,
			expectedTierOrder: []string{"platform"},
			expectedServices:  map[string][]string{"platform": {"api", "web"}},
		},
		{
			name: "mixed override and inherited tiers",
			yaml: `services:
  db:
    tier: foundation
  api:
    dir: ./api
  cache:
    dir: ./cache
defaults:
  tier: platform`,
			expectedTierOrder: []string{"foundation", "platform"},
			expectedServices:  map[string][]string{"foundation": {"db"}, "platform": {"api", "cache"}},
		},
		{
			name: "defaults.tier with whitespace and case",
			yaml: `services:
  api:
    dir: ./api
defaults:
  tier: " PLATFORM "`,
			expectedTierOrder: []string{"platform"},
			expectedServices:  map[string][]string{"platform": {"api"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topology, err := parseTierOrder([]byte(tt.yaml))
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTierOrder, topology.Order)
			assert.Equal(t, tt.expectedServices, topology.TierServices)
		})
	}
}

func Test_ParseTierOrder_HasDefaultOnly(t *testing.T) {
	tests := []struct {
		name                   string
		yaml                   string
		expectedHasDefaultOnly bool
	}{
		{
			name:                   "empty services has default only",
			yaml:                   `services: {}`,
			expectedHasDefaultOnly: true,
		},
		{
			name: "services without tiers has default only",
			yaml: `services:
  api:
    dir: ./api`,
			expectedHasDefaultOnly: true,
		},
		{
			name: "services with only default tier has default only",
			yaml: `services:
  api:
    tier: default`,
			expectedHasDefaultOnly: true,
		},
		{
			name: "services with multiple tiers not default only",
			yaml: `services:
  db:
    tier: foundation
  api:
    tier: platform`,
			expectedHasDefaultOnly: false,
		},
		{
			name: "services with foundation tier not default only",
			yaml: `services:
  db:
    tier: foundation`,
			expectedHasDefaultOnly: false,
		},
		{
			name: "mixed override and default not default only",
			yaml: `services:
  db:
    tier: foundation
  api:
    dir: ./api`,
			expectedHasDefaultOnly: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topology, err := parseTierOrder([]byte(tt.yaml))
			require.NoError(t, err)
			assert.Equal(t, tt.expectedHasDefaultOnly, topology.HasDefaultOnly)
		})
	}
}
