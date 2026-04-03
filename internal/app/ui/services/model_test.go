package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetTotalServices(t *testing.T) {
	tests := []struct {
		name  string
		tiers []Tier
		want  int
	}{
		{
			name:  "empty tiers",
			tiers: []Tier{},
			want:  0,
		},
		{
			name:  "single tier single service",
			tiers: []Tier{{Services: []string{"api"}}},
			want:  1,
		},
		{
			name:  "single tier multiple services",
			tiers: []Tier{{Services: []string{"api", "db", "cache"}}},
			want:  3,
		},
		{
			name:  "multiple tiers",
			tiers: []Tier{{Services: []string{"db"}}, {Services: []string{"api", "web"}}},
			want:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.tiers = tt.tiers
			assert.Equal(t, tt.want, m.getTotalServices())
		})
	}
}

func Test_GetReadyServices(t *testing.T) {
	tests := []struct {
		name     string
		services map[string]*ServiceState
		want     int
	}{
		{
			name:     "empty services",
			services: map[string]*ServiceState{},
			want:     0,
		},
		{
			name:     "no ready services",
			services: map[string]*ServiceState{"api": {Status: StatusStarting}, "db": {Status: StatusFailed}},
			want:     0,
		},
		{
			name:     "some ready",
			services: map[string]*ServiceState{"api": {Status: StatusRunning}, "db": {Status: StatusStarting}},
			want:     1,
		},
		{
			name:     "all ready",
			services: map[string]*ServiceState{"api": {Status: StatusRunning}, "db": {Status: StatusRunning}},
			want:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.services = tt.services
			assert.Equal(t, tt.want, m.getReadyServices())
		})
	}
}

func Test_GetSelectedService(t *testing.T) {
	services := map[string]*ServiceState{
		"db":  {Name: "db"},
		"api": {Name: "api"},
		"web": {Name: "web"},
	}
	tiers := []Tier{
		{Name: "tier1", Services: []string{"db"}},
		{Name: "tier2", Services: []string{"api", "web"}},
	}

	tests := []struct {
		name     string
		selected int
		want     string
	}{
		{
			name:     "first service",
			selected: 0,
			want:     "db",
		},
		{
			name:     "second service",
			selected: 1,
			want:     "api",
		},
		{
			name:     "third service",
			selected: 2,
			want:     "web",
		},
		{
			name:     "negative index",
			selected: -1,
			want:     "",
		},
		{
			name:     "out of bounds",
			selected: 10,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.tiers = tiers
			m.state.services = services
			m.state.selected = tt.selected

			result := m.getSelectedService()
			if tt.want == "" {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.want, result.Name)
			}
		})
	}
}

func Test_CalculateScrollOffset(t *testing.T) {
	tests := []struct {
		name     string
		tiers    []Tier
		services map[string]*ServiceState
		selected int
		vpHeight int
		vpOffset int
		assertFn func(t *testing.T, offset int)
	}{
		{
			name:     "zero height viewport returns current offset",
			tiers:    []Tier{{Services: []string{"api"}}},
			services: map[string]*ServiceState{"api": {Name: "api"}},
			selected: 0,
			vpHeight: 0,
			assertFn: func(t *testing.T, offset int) {
				assert.Equal(t, 0, offset)
			},
		},
		{
			name:     "selection visible returns current offset",
			tiers:    []Tier{{Services: []string{"api", "db"}}},
			services: map[string]*ServiceState{"api": {Name: "api"}, "db": {Name: "db"}},
			selected: 0,
			vpHeight: 10,
			assertFn: func(t *testing.T, offset int) {
				assert.Equal(t, 0, offset)
			},
		},
		{
			name: "multi-tier selection visible",
			tiers: []Tier{
				{Services: []string{"a", "b"}},
				{Services: []string{"c", "d"}},
			},
			services: map[string]*ServiceState{
				"a": {Name: "a"}, "b": {Name: "b"},
				"c": {Name: "c"}, "d": {Name: "d"},
			},
			selected: 3,
			vpHeight: 20,
			assertFn: func(t *testing.T, offset int) {
				assert.Equal(t, 0, offset)
			},
		},
		{
			name:     "scrolls down when selection below viewport",
			tiers:    []Tier{{Services: []string{"a", "b", "c", "d", "e", "f"}}},
			services: map[string]*ServiceState{"a": {}, "b": {}, "c": {}, "d": {}, "e": {}, "f": {}},
			selected: 5,
			vpHeight: 3,
			vpOffset: 0,
			assertFn: func(t *testing.T, offset int) {
				assert.Positive(t, offset)
			},
		},
		{
			name:     "scrolls up when selection above viewport",
			tiers:    []Tier{{Services: []string{"a", "b", "c", "d", "e"}}},
			services: map[string]*ServiceState{"a": {}, "b": {}, "c": {}, "d": {}, "e": {}},
			selected: 0,
			vpHeight: 3,
			vpOffset: 10,
			assertFn: func(t *testing.T, offset int) {
				assert.Less(t, offset, 10)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.tiers = tt.tiers
			m.state.services = tt.services
			m.state.selected = tt.selected
			m.ui.servicesViewport.SetHeight(tt.vpHeight)

			if tt.vpOffset > 0 {
				m.ui.servicesViewport.SetYOffset(tt.vpOffset)
			}

			offset := m.calculateScrollOffset()
			tt.assertFn(t, offset)
		})
	}
}
