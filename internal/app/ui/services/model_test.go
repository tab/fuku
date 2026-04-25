package services

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
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

func Test_RefreshFromStore_MetricsUpdatedWithMatchingLifecycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:     "id-api",
			CPU:    25.5,
			Memory: 256 * 1024 * 1024,
		},
		{
			ID:     "id-unknown",
			CPU:    10.0,
			Memory: 128 * 1024 * 1024,
		},
	})

	m := &Model{store: mockStore, loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.restarting = map[string]bool{}
	m.state.services = map[string]*ServiceState{
		"id-api": {Name: "api"},
	}

	m.refreshFromStore()

	api := m.state.services["id-api"]
	assert.InDelta(t, 25.5, api.CPU, 0.01)
	assert.InDelta(t, 256.0, api.MEM, 0.01)
}

func Test_RefreshFromStore_NewerSnapshotHealsLifecycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:               "id-api",
			Status:           registry.StatusRunning,
			PID:              9999,
			CPU:              50.0,
			Memory:           512 * 1024 * 1024,
			StartTime:        t0,
			AttemptStartedAt: t0,
			LifecycleAt:      t1,
			LifecycleSeq:     2,
			Watching:         true,
			WatchAt:          t1,
			WatchSeq:         2,
		},
	})

	m := &Model{store: mockStore, loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.restarting = map[string]bool{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			Name:         "api",
			Status:       StatusStarting,
			PID:          1234,
			LifecycleAt:  t0,
			LifecycleSeq: 1,
			WatchAt:      t0,
			WatchSeq:     1,
		},
	}

	m.refreshFromStore()

	api := m.state.services["id-api"]
	assert.Equal(t, registry.StatusRunning, api.Status)
	assert.Equal(t, 9999, api.PID)
	assert.Equal(t, t0, api.StartTime)
	assert.True(t, api.Watching)
	assert.Equal(t, t1, api.LifecycleAt)
	assert.Equal(t, t1, api.WatchAt)
	assert.InDelta(t, 0.0, api.CPU, 0.01)
	assert.InDelta(t, 0.0, api.MEM, 0.01)
}

func Test_RefreshFromStore_OlderSnapshotDoesNotOverwrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusStarting,
			PID:          1234,
			CPU:          50.0,
			Memory:       512 * 1024 * 1024,
			StartTime:    t0,
			LifecycleAt:  t0,
			LifecycleSeq: 1,
			Watching:     false,
			WatchAt:      t0,
			WatchSeq:     1,
		},
	})

	m := &Model{store: mockStore, loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.restarting = map[string]bool{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			Name:         "api",
			Status:       StatusFailed,
			PID:          0,
			Error:        assert.AnError,
			LifecycleAt:  t1,
			LifecycleSeq: 2,
			Watching:     true,
			WatchAt:      t1,
			WatchSeq:     2,
		},
	}

	m.refreshFromStore()

	api := m.state.services["id-api"]
	assert.Equal(t, StatusFailed, api.Status)
	assert.Equal(t, 0, api.PID)
	require.EqualError(t, api.Error, assert.AnError.Error())
	assert.True(t, api.Watching)
	assert.InDelta(t, 0.0, api.CPU, 0.01)
	assert.InDelta(t, 0.0, api.MEM, 0.01)
}

func Test_RefreshFromStore_DroppedWatchEventHealed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:       "id-api",
			Watching: true,
			WatchAt:  t1,
			WatchSeq: 5,
		},
	})

	m := &Model{store: mockStore, loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.restarting = map[string]bool{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			Name:     "api",
			WatchAt:  t0,
			WatchSeq: 3,
		},
	}

	m.refreshFromStore()

	assert.True(t, m.state.services["id-api"].Watching)
	assert.Equal(t, t1, m.state.services["id-api"].WatchAt)
	assert.Equal(t, uint64(5), m.state.services["id-api"].WatchSeq)
}

func Test_RefreshFromStore_SameSeqSameStatusHealsFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusRunning,
			PID:          5678,
			StartTime:    t0,
			LifecycleAt:  t0,
			LifecycleSeq: 10,
		},
	})

	m := &Model{store: mockStore, loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.restarting = map[string]bool{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			Name:         "api",
			Status:       StatusRunning,
			PID:          0,
			StartTime:    time.Time{},
			Error:        assert.AnError,
			LifecycleAt:  t0,
			LifecycleSeq: 10,
		},
	}

	m.refreshFromStore()

	api := m.state.services["id-api"]
	assert.Equal(t, registry.StatusRunning, api.Status)
	assert.Equal(t, 5678, api.PID)
	assert.Equal(t, t0, api.StartTime)
	assert.NoError(t, api.Error)
}

func Test_RefreshFromStore_SameSeqDifferentStatusDoesNotOverwrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusStarting,
			PID:          1234,
			StartTime:    t0,
			LifecycleAt:  t0,
			LifecycleSeq: 10,
		},
	})

	m := &Model{store: mockStore, loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.restarting = map[string]bool{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			Name:         "api",
			Status:       StatusRunning,
			PID:          5678,
			LifecycleAt:  t0,
			LifecycleSeq: 10,
		},
	}

	m.refreshFromStore()

	api := m.state.services["id-api"]
	assert.Equal(t, StatusRunning, api.Status)
	assert.Equal(t, 5678, api.PID)
}

func Test_RefreshFromStore_RestartingClearsStartTime(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:               "id-api",
			Status:           registry.StatusRestarting,
			PID:              0,
			StartTime:        time.Time{},
			AttemptStartedAt: t0,
			LifecycleAt:      t0.Add(10 * time.Second),
			LifecycleSeq:     15,
		},
	})

	m := &Model{store: mockStore, loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.restarting = map[string]bool{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			Name:             "api",
			Status:           StatusRunning,
			PID:              1234,
			StartTime:        t0,
			AttemptStartedAt: t0,
			LifecycleAt:      t0.Add(5 * time.Second),
			LifecycleSeq:     10,
			Timeline:         NewTimeline(20),
		},
	}

	m.refreshFromStore()

	api := m.state.services["id-api"]
	assert.Equal(t, StatusRestarting, api.Status)
	assert.True(t, api.StartTime.IsZero(), "StartTime must be blank during restart")
	assert.Equal(t, t0, api.AttemptStartedAt, "AttemptStartedAt must be preserved during restart")
}

func Test_RefreshFromStore_TerminalRetryPrefersSnapshotAttempt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	oldAttempt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newAttempt := oldAttempt.Add(10 * time.Second)
	failedAt := newAttempt.Add(2 * time.Second)

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:               "id-api",
			Status:           registry.StatusFailed,
			AttemptStartedAt: newAttempt,
			LifecycleAt:      failedAt,
			LifecycleSeq:     20,
		},
	})

	tl := NewTimeline(20)
	m := &Model{store: mockStore, loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}}
	m.state.restarting = map[string]bool{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			Name:             "api",
			Status:           StatusStarting,
			StartupActive:    true,
			AttemptStartedAt: oldAttempt,
			LifecycleSeq:     10,
			Timeline:         tl,
		},
	}

	m.refreshFromStore()

	assert.Equal(t, 2, tl.Count(), "backfill should use 2s snapshot attempt, not stale UI attempt")

	slots := tl.Slots()
	for i := range 2 {
		assert.Equal(t, SlotStarting, slots[i])
	}
}

func Test_RefreshFromStore_UnchangedSnapshotKeepsOptimisticLoader(t *testing.T) {
	tests := []struct {
		name   string
		status registry.Status
	}{
		{
			name:   "stopped service with start loader",
			status: registry.StatusStopped,
		},
		{
			name:   "running service with stop loader",
			status: registry.StatusRunning,
		},
		{
			name:   "failed service with restart loader",
			status: registry.StatusFailed,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := registry.NewMockStore(ctrl)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
				{
					ID:           "id-api",
					Status:       tt.status,
					LifecycleSeq: 5,
				},
			})

			loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
			m := &Model{store: mockStore, loader: loader}
			m.state.restarting = map[string]bool{}
			m.state.services = map[string]*ServiceState{
				"id-api": {
					Name:         "api",
					Status:       tt.status,
					LifecycleSeq: 5,
				},
			}

			loader.Start("id-api", "working…")

			m.refreshFromStore()

			assert.True(t, loader.Active, "optimistic loader must survive unchanged snapshot")
		})
	}
}

func Test_RefreshFromStore_NewerSnapshotStopsLoader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := registry.NewMockStore(ctrl)
	mockStore.EXPECT().Services().Return([]registry.ServiceSnapshot{
		{
			ID:           "id-api",
			Status:       registry.StatusRunning,
			LifecycleSeq: 10,
		},
	})

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := &Model{store: mockStore, loader: loader}
	m.state.restarting = map[string]bool{}
	m.state.services = map[string]*ServiceState{
		"id-api": {
			ID:           "id-api",
			Name:         "api",
			Status:       StatusStarting,
			LifecycleSeq: 5,
		},
	}

	loader.Start("id-api", "starting…")

	m.refreshFromStore()

	assert.False(t, loader.Active, "newer lifecycle snapshot must stop loader")
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
