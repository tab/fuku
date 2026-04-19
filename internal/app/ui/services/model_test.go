package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/registry"
)

func Test_GetTotalServices(t *testing.T) {
	tests := []struct {
		name        string
		serviceIDs  []string
		filteredIDs []string
		filterQuery string
		want        int
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
		{
			name:        "uses filtered IDs when filtering",
			serviceIDs:  []string{"id-api", "id-db", "id-cache"},
			filteredIDs: []string{"id-api"},
			filterQuery: "api",
			want:        1,
		},
		{
			name:        "empty filter query uses all IDs",
			serviceIDs:  []string{"id-api", "id-db"},
			filteredIDs: []string{"id-api"},
			filterQuery: "",
			want:        2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.serviceIDs = tt.serviceIDs
			m.state.filteredIDs = tt.filteredIDs
			m.state.filterQuery = tt.filterQuery
			assert.Equal(t, tt.want, m.getTotalServices())
		})
	}
}

func Test_GetReadyServices(t *testing.T) {
	tests := []struct {
		name       string
		serviceIDs []string
		services   map[string]*ServiceState
		want       int
	}{
		{
			name:       "empty services",
			serviceIDs: []string{},
			services:   map[string]*ServiceState{},
			want:       0,
		},
		{
			name:       "no ready services",
			serviceIDs: []string{"id-api", "id-db"},
			services:   map[string]*ServiceState{"id-api": {Status: StatusStarting}, "id-db": {Status: StatusFailed}},
			want:       0,
		},
		{
			name:       "some ready",
			serviceIDs: []string{"id-api", "id-db"},
			services:   map[string]*ServiceState{"id-api": {Status: StatusRunning}, "id-db": {Status: StatusStarting}},
			want:       1,
		},
		{
			name:       "all ready",
			serviceIDs: []string{"id-api", "id-db"},
			services:   map[string]*ServiceState{"id-api": {Status: StatusRunning}, "id-db": {Status: StatusRunning}},
			want:       2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.serviceIDs = tt.serviceIDs
			m.state.services = tt.services
			assert.Equal(t, tt.want, m.getReadyServices())
		})
	}
}

func Test_GetAllReadyServices(t *testing.T) {
	tests := []struct {
		name        string
		serviceIDs  []string
		services    map[string]*ServiceState
		filterQuery string
		filteredIDs []string
		want        int
	}{
		{
			name:       "counts all ready services",
			serviceIDs: []string{"id-api", "id-db"},
			services: map[string]*ServiceState{
				"id-api": {Status: StatusRunning},
				"id-db":  {Status: StatusRunning},
			},
			want: 2,
		},
		{
			name:       "counts only running services",
			serviceIDs: []string{"id-api", "id-db"},
			services: map[string]*ServiceState{
				"id-api": {Status: StatusRunning},
				"id-db":  {Status: StatusFailed},
			},
			want: 1,
		},
		{
			name:        "ignores filter and counts all services",
			serviceIDs:  []string{"id-api", "id-db", "id-web"},
			filteredIDs: []string{"id-api"},
			filterQuery: "api",
			services: map[string]*ServiceState{
				"id-api": {Status: StatusRunning},
				"id-db":  {Status: StatusRunning},
				"id-web": {Status: StatusFailed},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.serviceIDs = tt.serviceIDs
			m.state.services = tt.services
			m.state.filterQuery = tt.filterQuery
			m.state.filteredIDs = tt.filteredIDs
			assert.Equal(t, tt.want, m.getAllReadyServices())
		})
	}
}

func Test_IsFiltering(t *testing.T) {
	tests := []struct {
		name        string
		filterQuery string
		want        bool
	}{
		{
			name:        "empty query",
			filterQuery: "",
			want:        false,
		},
		{
			name:        "non-empty query",
			filterQuery: "api",
			want:        true,
		},
		{
			name:        "whitespace only normalizes to empty",
			filterQuery: " ",
			want:        false,
		},
		{
			name:        "separator only normalizes to empty",
			filterQuery: "---",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.filterQuery = tt.filterQuery
			assert.Equal(t, tt.want, m.isFiltering())
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
		name        string
		selected    int
		filterQuery string
		filteredIDs []string
		want        string
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
		{
			name:        "uses filtered IDs when filtering",
			selected:    0,
			filterQuery: "web",
			filteredIDs: []string{"id-web"},
			want:        "web",
		},
		{
			name:        "out of bounds in filtered list",
			selected:    5,
			filterQuery: "web",
			filteredIDs: []string{"id-web"},
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			m.state.services = services
			m.state.serviceIDs = serviceIDs
			m.state.selected = tt.selected
			m.state.filterQuery = tt.filterQuery
			m.state.filteredIDs = tt.filteredIDs

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

func Test_RefreshFromStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:        "id-api",
			Status:    registry.StatusRunning,
			PID:       1234,
			CPU:       25.5,
			Memory:    256 * 1024 * 1024,
			StartTime: now,
			Watching:  true,
		},
		{
			ID:     "id-db",
			Status: registry.StatusFailed,
			Error:  "connection refused",
		},
		{
			ID:     "id-unknown",
			Status: registry.StatusRunning,
		},
	})

	m := &Model{store: mockStore}
	m.state.services = map[string]*ServiceState{
		"id-api": {Name: "api"},
		"id-db":  {Name: "db"},
	}

	m.refreshFromStore()

	api := m.state.services["id-api"]
	assert.Equal(t, registry.StatusRunning, api.Status)
	assert.Equal(t, 1234, api.PID)
	assert.InDelta(t, 25.5, api.CPU, 0.01)
	assert.InDelta(t, 256.0, api.MEM, 0.01)
	assert.Equal(t, now, api.StartTime)
	assert.True(t, api.Watching)
	require.NoError(t, api.Error)

	db := m.state.services["id-db"]
	assert.Equal(t, registry.StatusFailed, db.Status)
	assert.EqualError(t, db.Error, "connection refused")
}

func Test_RefreshFromStore_ClearsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:     "id-api",
			Status: registry.StatusRunning,
			Error:  "",
		},
	})

	m := &Model{store: mockStore}
	m.state.services = map[string]*ServiceState{
		"id-api": {Name: "api", Error: assert.AnError},
	}

	m.refreshFromStore()

	require.NoError(t, m.state.services["id-api"].Error)
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
