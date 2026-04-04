package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetTotalServices(t *testing.T) {
	tests := []struct {
		name       string
		serviceIDs []string
		want       int
	}{
		{
			name:       "empty",
			serviceIDs: nil,
			want:       0,
		},
		{
			name:       "single service",
			serviceIDs: []string{"id-api"},
			want:       1,
		},
		{
			name:       "multiple services",
			serviceIDs: []string{"id-api", "id-db", "id-cache"},
			want:       3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.serviceIDs = tt.serviceIDs
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
			services: map[string]*ServiceState{"id-api": {Status: StatusStarting}, "id-db": {Status: StatusFailed}},
			want:     0,
		},
		{
			name:     "some ready",
			services: map[string]*ServiceState{"id-api": {Status: StatusRunning}, "id-db": {Status: StatusStarting}},
			want:     1,
		},
		{
			name:     "all ready",
			services: map[string]*ServiceState{"id-api": {Status: StatusRunning}, "id-db": {Status: StatusRunning}},
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
		"id-db":  {ID: "id-db", Name: "db"},
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
	}
	serviceIDs := []string{"id-db", "id-api", "id-web"}

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
			m.state.services = services
			m.state.serviceIDs = serviceIDs
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
			tiers:    []Tier{{Services: []string{"id-api"}}},
			services: map[string]*ServiceState{"id-api": {Name: "api"}},
			selected: 0,
			vpHeight: 0,
			assertFn: func(t *testing.T, offset int) {
				assert.Equal(t, 0, offset)
			},
		},
		{
			name:     "selection visible returns current offset",
			tiers:    []Tier{{Services: []string{"id-api", "id-db"}}},
			services: map[string]*ServiceState{"id-api": {Name: "api"}, "id-db": {Name: "db"}},
			selected: 0,
			vpHeight: 10,
			assertFn: func(t *testing.T, offset int) {
				assert.Equal(t, 0, offset)
			},
		},
		{
			name: "multi-tier selection visible",
			tiers: []Tier{
				{Services: []string{"id-a", "id-b"}},
				{Services: []string{"id-c", "id-d"}},
			},
			services: map[string]*ServiceState{
				"id-a": {Name: "a"}, "id-b": {Name: "b"},
				"id-c": {Name: "c"}, "id-d": {Name: "d"},
			},
			selected: 3,
			vpHeight: 20,
			assertFn: func(t *testing.T, offset int) {
				assert.Equal(t, 0, offset)
			},
		},
		{
			name:     "scrolls down when selection below viewport",
			tiers:    []Tier{{Services: []string{"id-a", "id-b", "id-c", "id-d", "id-e", "id-f"}}},
			services: map[string]*ServiceState{"id-a": {}, "id-b": {}, "id-c": {}, "id-d": {}, "id-e": {}, "id-f": {}},
			selected: 5,
			vpHeight: 3,
			vpOffset: 0,
			assertFn: func(t *testing.T, offset int) {
				assert.Positive(t, offset)
			},
		},
		{
			name:     "scrolls up when selection above viewport",
			tiers:    []Tier{{Services: []string{"id-a", "id-b", "id-c", "id-d", "id-e"}}},
			services: map[string]*ServiceState{"id-a": {}, "id-b": {}, "id-c": {}, "id-d": {}, "id-e": {}},
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
