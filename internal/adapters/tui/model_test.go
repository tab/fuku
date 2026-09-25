package tui

import (
	"log/slog"
	"strings"
	"testing"

	"charm.land/bubbles/v2/help"
	"github.com/stretchr/testify/assert"

	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
)

// readFrom stands in for Registry.Read and runs the callback on the fixture snapshot
func readFrom(snapshot *model.Snapshot) func(func(*model.Snapshot)) {
	return func(fn func(*model.Snapshot)) {
		fn(snapshot)
	}
}

func Test_NewModel(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	log := slog.New(slog.DiscardHandler)

	m := NewModel(t.Context(), ModelParams{Profile: "dev", Project: model.Project{}, Theme: theme, Logger: log})

	assert.Equal(t, theme, m.theme)
	assert.Equal(t, help.DefaultStyles(true), m.ui.help.Styles)
	assert.Equal(t, "dev", m.state.profile)
	assert.Empty(t, m.state.views)
	assert.NotNil(t, m.loader)
}

func Test_ActiveServiceIDs(t *testing.T) {
	tests := []struct {
		name   string
		before func() Model
		want   int
	}{
		{
			name: "empty",
			before: func() Model {
				return Model{}
			},
			want: 0,
		},
		{
			name: "single service",
			before: func() Model {
				m := Model{}
				m.state.serviceIDs = []string{"id-api"}

				return m
			},
			want: 1,
		},
		{
			name: "multiple services",
			before: func() Model {
				m := Model{}
				m.state.serviceIDs = []string{"id-api", "id-db", "id-cache"}

				return m
			},
			want: 3,
		},
		{
			name: "uses filtered IDs when filtering",
			before: func() Model {
				m := Model{}
				m.state.serviceIDs = []string{"id-api", "id-db", "id-cache"}
				m.state.filteredIDs = []string{"id-api"}
				m.state.filterQuery = "api"

				return m
			},
			want: 1,
		},
		{
			name: "empty filter query uses all IDs",
			before: func() Model {
				m := Model{}
				m.state.serviceIDs = []string{"id-api", "id-db"}
				m.state.filteredIDs = []string{"id-api"}
				m.state.filterQuery = ""

				return m
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := len(m.activeServiceIDs())

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_GetAllReadyServices(t *testing.T) {
	tests := []struct {
		name   string
		before func() Model
		want   int
	}{
		{
			name: "counts all ready services",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Services: map[string]*model.Service{
					"id-api": {Status: model.StatusRunning},
					"id-db":  {Status: model.StatusRunning},
				}}}
				m.state.serviceIDs = []string{"id-api", "id-db"}

				return m
			},
			want: 2,
		},
		{
			name: "counts only running services",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Services: map[string]*model.Service{
					"id-api": {Status: model.StatusRunning},
					"id-db":  {Status: model.StatusFailed},
				}}}
				m.state.serviceIDs = []string{"id-api", "id-db"}

				return m
			},
			want: 1,
		},
		{
			name: "ignores filter and counts all services",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Services: map[string]*model.Service{
					"id-api": {Status: model.StatusRunning},
					"id-db":  {Status: model.StatusRunning},
					"id-web": {Status: model.StatusFailed},
				}}}
				m.state.serviceIDs = []string{"id-api", "id-db", "id-web"}
				m.state.filteredIDs = []string{"id-api"}
				m.state.filterQuery = "api"

				return m
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.getAllReadyServices()

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_IsFiltering(t *testing.T) {
	tests := []struct {
		name   string
		before func() Model
		want   bool
	}{
		{
			name: "empty query",
			before: func() Model {
				m := Model{}
				m.state.filterQuery = ""

				return m
			},
			want: false,
		},
		{
			name: "non-empty query",
			before: func() Model {
				m := Model{}
				m.state.filterQuery = "api"

				return m
			},
			want: true,
		},
		{
			name: "whitespace only normalizes to empty",
			before: func() Model {
				m := Model{}
				m.state.filterQuery = " "

				return m
			},
			want: false,
		},
		{
			name: "separator only normalizes to empty",
			before: func() Model {
				m := Model{}
				m.state.filterQuery = "---"

				return m
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.isFiltering()

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_GetSelectedService(t *testing.T) {
	db := &model.Service{ID: "id-db", Name: "db"}
	api := &model.Service{ID: "id-api", Name: "api"}
	web := &model.Service{ID: "id-web", Name: "web"}
	snapshot := &model.Snapshot{Services: map[string]*model.Service{"id-db": db, "id-api": api, "id-web": web}}
	serviceIDs := []string{"id-db", "id-api", "id-web"}

	tests := []struct {
		name   string
		before func() Model
		want   *model.Service
	}{
		{
			name: "first service",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.serviceIDs = serviceIDs
				m.state.selected = 0

				return m
			},
			want: db,
		},
		{
			name: "second service",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.serviceIDs = serviceIDs
				m.state.selected = 1

				return m
			},
			want: api,
		},
		{
			name: "third service",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.serviceIDs = serviceIDs
				m.state.selected = 2

				return m
			},
			want: web,
		},
		{
			name: "negative index",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.serviceIDs = serviceIDs
				m.state.selected = -1

				return m
			},
			want: nil,
		},
		{
			name: "out of bounds",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.serviceIDs = serviceIDs
				m.state.selected = 10

				return m
			},
			want: nil,
		},
		{
			name: "uses filtered IDs when filtering",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.serviceIDs = serviceIDs
				m.state.selected = 0
				m.state.filterQuery = "web"
				m.state.filteredIDs = []string{"id-web"}

				return m
			},
			want: web,
		},
		{
			name: "out of bounds in filtered list",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.serviceIDs = serviceIDs
				m.state.selected = 5
				m.state.filterQuery = "web"
				m.state.filteredIDs = []string{"id-web"}

				return m
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.getSelectedService()

			assert.Same(t, tt.want, result)
		})
	}
}

func Test_CalculateScrollOffset(t *testing.T) {
	content := strings.Repeat("\n", 20)

	tests := []struct {
		name     string
		before   func() Model
		expected int
	}{
		{
			name: "zero height viewport returns current offset",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Tiers: []*model.Tier{{Services: []*model.Service{{Name: "api"}}}}}}
				m.state.selected = 0
				m.ui.servicesViewport.SetHeight(0)
				m.ui.servicesViewport.SetContent(content)

				return m
			},
			expected: 0,
		},
		{
			name: "selection visible returns current offset",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Tiers: []*model.Tier{{Services: []*model.Service{{Name: "api"}, {Name: "db"}}}}}}
				m.state.selected = 0
				m.ui.servicesViewport.SetHeight(10)
				m.ui.servicesViewport.SetContent(content)

				return m
			},
			expected: 0,
		},
		{
			name: "multi-tier selection visible",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Tiers: []*model.Tier{
					{Services: []*model.Service{{Name: "a"}, {Name: "b"}}},
					{Services: []*model.Service{{Name: "c"}, {Name: "d"}}},
				}}}
				m.state.selected = 3
				m.ui.servicesViewport.SetHeight(20)
				m.ui.servicesViewport.SetContent(content)

				return m
			},
			expected: 0,
		},
		{
			name: "scrolls down when selection below viewport",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Tiers: []*model.Tier{{Services: []*model.Service{{}, {}, {}, {}, {}, {}}}}}}
				m.state.selected = 5
				m.ui.servicesViewport.SetHeight(3)
				m.ui.servicesViewport.SetContent(content)
				m.ui.servicesViewport.SetYOffset(0)

				return m
			},
			expected: 5,
		},
		{
			name: "scrolls up when selection above viewport",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Tiers: []*model.Tier{{Services: []*model.Service{{}, {}, {}, {}, {}}}}}}
				m.state.selected = 0
				m.ui.servicesViewport.SetHeight(3)
				m.ui.servicesViewport.SetContent(content)
				m.ui.servicesViewport.SetYOffset(10)

				return m
			},
			expected: 1,
		},
		{
			name: "scrolls up to the selection when it is not first in its tier",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Tiers: []*model.Tier{{Services: []*model.Service{{}, {}, {}, {}, {}, {}}}}}}
				m.state.selected = 3
				m.ui.servicesViewport.SetHeight(3)
				m.ui.servicesViewport.SetContent(content)
				m.ui.servicesViewport.SetYOffset(10)

				return m
			},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			offset := m.calculateScrollOffset()

			assert.Equal(t, tt.expected, offset)
		})
	}
}
