package tui

import (
	"testing"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"fuku/internal/model"
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
	services := map[string]*model.Service{
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

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_FilterTiers(t *testing.T) {
	db := &model.Service{ID: "id-db", Name: "database"}
	cache := &model.Service{ID: "id-cache", Name: "cache-server"}
	api := &model.Service{ID: "id-api", Name: "api-server"}
	web := &model.Service{ID: "id-web", Name: "web-app"}
	worker := &model.Service{ID: "id-worker", Name: "worker"}
	tiers := []*model.Tier{
		{Name: "foundation", Ready: true, Services: []*model.Service{db, cache}},
		{Name: "application", Ready: false, Services: []*model.Service{api, web}},
		{Name: "background", Ready: true, Services: []*model.Service{worker}},
	}

	tests := []struct {
		name     string
		query    string
		expected []*model.Tier
	}{
		{
			name:     "empty query returns all tiers",
			query:    "",
			expected: tiers,
		},
		{
			name:  "match across tiers",
			query: "server",
			expected: []*model.Tier{
				{Name: "foundation", Ready: true, Services: []*model.Service{cache}},
				{Name: "application", Ready: false, Services: []*model.Service{api}},
			},
		},
		{
			name:  "empty tier omitted",
			query: "web",
			expected: []*model.Tier{
				{Name: "application", Ready: false, Services: []*model.Service{web}},
			},
		},
		{
			name:     "no matches returns empty",
			query:    "nosuchservice",
			expected: []*model.Tier{},
		},
		{
			name:  "single tier match",
			query: "worker",
			expected: []*model.Tier{
				{Name: "background", Ready: true, Services: []*model.Service{worker}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterTiers(tt.query, tiers)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_HandleFilterKey(t *testing.T) {
	m := Model{}
	m.state.filterActive = false
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.state.selected = 2
	m.ui.servicesKeys = defaultKeyMap()

	result, cmd := m.handleFilterKey()

	assert.True(t, result.state.filterActive)
	assert.Equal(t, "id-db", result.state.preFilterSelectedID)
	assert.Nil(t, cmd)
}

func Test_HandleFilterInput(t *testing.T) {
	api := &model.Service{ID: "id-api", Name: "api"}
	web := &model.Service{ID: "id-web", Name: "web"}
	db := &model.Service{ID: "id-db", Name: "db"}
	snapshot := &model.Snapshot{
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api, web, db}}},
		Services: map[string]*model.Service{"id-api": api, "id-web": web, "id-db": db},
	}

	tests := []struct {
		name           string
		before         func() Model
		msg            tea.KeyPressMsg
		expectedQuery  string
		expectedActive bool
	}{
		{
			name: "Typing appends to query",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.filterQuery = "ap"
				m.state.filterActive = true
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}

				return m
			},
			msg:            tea.KeyPressMsg{Code: 'i', Text: "i"},
			expectedQuery:  "api",
			expectedActive: true,
		},
		{
			name: "Typing first character",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.filterQuery = ""
				m.state.filterActive = true
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}

				return m
			},
			msg:            tea.KeyPressMsg{Code: 'a', Text: "a"},
			expectedQuery:  "a",
			expectedActive: true,
		},
		{
			name: "Backspace removes last character",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.filterQuery = "api"
				m.state.filterActive = true
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}

				return m
			},
			msg:            tea.KeyPressMsg{Code: tea.KeyBackspace},
			expectedQuery:  "ap",
			expectedActive: true,
		},
		{
			name: "Backspace on empty query is no-op",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.filterQuery = ""
				m.state.filterActive = true
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}

				return m
			},
			msg:            tea.KeyPressMsg{Code: tea.KeyBackspace},
			expectedQuery:  "",
			expectedActive: true,
		},
		{
			name: "Enter exits input mode keeping filter",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.filterQuery = "api"
				m.state.filterActive = true
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}

				return m
			},
			msg:            tea.KeyPressMsg{Code: tea.KeyEnter},
			expectedQuery:  "api",
			expectedActive: false,
		},
		{
			name: "Escape clears filter and exits input mode",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.filterQuery = "api"
				m.state.filterActive = true
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}

				return m
			},
			msg:            tea.KeyPressMsg{Code: tea.KeyEscape},
			expectedQuery:  "",
			expectedActive: false,
		},
		{
			name: "Arrow up does not modify query",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.filterQuery = "api"
				m.state.filterActive = true
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}

				return m
			},
			msg:            tea.KeyPressMsg{Code: tea.KeyUp},
			expectedQuery:  "api",
			expectedActive: true,
		},
		{
			name: "Arrow down does not modify query",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.filterQuery = "api"
				m.state.filterActive = true
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}

				return m
			},
			msg:            tea.KeyPressMsg{Code: tea.KeyDown},
			expectedQuery:  "api",
			expectedActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, _ := m.handleFilterInput(tt.msg)

			assert.Equal(t, tt.expectedQuery, result.state.filterQuery)
			assert.Equal(t, tt.expectedActive, result.state.filterActive)
		})
	}
}

func Test_ApplyFilter_SelectionAdjustment(t *testing.T) {
	snapshot := &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api-server"},
		"id-web": {ID: "id-web", Name: "web-app"},
		"id-db":  {ID: "id-db", Name: "db-primary"},
	}}

	tests := []struct {
		name             string
		before           func() Model
		expectedSelected int
	}{
		{
			name: "Keeps selection when current service still visible",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
				m.state.filterQuery = "api"
				m.state.selected = 0

				return m
			},
			expectedSelected: 0,
		},
		{
			name: "Moves to first match when current service hidden",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
				m.state.filterQuery = "web"
				m.state.selected = 2

				return m
			},
			expectedSelected: 0,
		},
		{
			name: "Preserves selection at non-zero filtered position",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
				m.state.filterQuery = "p"
				m.state.selected = 1

				return m
			},
			expectedSelected: 1,
		},
		{
			name: "Resets to zero on no matches",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
				m.state.filterQuery = "zzz"
				m.state.selected = 2

				return m
			},
			expectedSelected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			m.applyFilter()

			assert.Equal(t, tt.expectedSelected, m.state.selected)
		})
	}
}

func Test_HandleFilterInput_ArrowDownNavigates(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = "b"
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"], m.snapshot.Services["id-db"]}},
	}
	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.selected = 0

	msg := tea.KeyPressMsg{Code: tea.KeyDown}

	result, _ := m.handleFilterInput(msg)

	assert.Equal(t, 1, result.state.selected)
	assert.Equal(t, "b", result.state.filterQuery)
	assert.True(t, result.state.filterActive)
}

func Test_HandleFilterInput_ArrowUpNavigates(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = "b"
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"], m.snapshot.Services["id-db"]}},
	}
	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.selected = 1

	msg := tea.KeyPressMsg{Code: tea.KeyUp}

	result, _ := m.handleFilterInput(msg)

	assert.Equal(t, 0, result.state.selected)
	assert.Equal(t, "b", result.state.filterQuery)
	assert.True(t, result.state.filterActive)
}

func Test_ApplyFilter_RestoresSelectionAfterTemporaryZeroMatches(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"], m.snapshot.Services["id-db"]}},
	}

	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.selected = 1
	m.state.filterQuery = "bx"
	m.applyFilter()

	assert.Empty(t, m.state.filteredIDs)
	assert.Equal(t, "id-db", m.state.lastFilteredSelectedID)

	m.state.filterQuery = "b"
	m.applyFilter()

	assert.Equal(t, []string{"id-web", "id-db"}, m.state.filteredIDs)
	assert.Equal(t, 1, m.state.selected)

	svc := m.snapshot.Services[m.state.filteredIDs[m.state.selected]]
	assert.Equal(t, "db", svc.Name)
}

func Test_ApplyFilter_TracksSelectionOnZeroMatches(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"], m.snapshot.Services["id-db"]}},
	}
	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.selected = 1
	m.state.filterQuery = "zzz"

	m.applyFilter()

	assert.Empty(t, m.state.filteredIDs)
	assert.Equal(t, "id-db", m.state.lastFilteredSelectedID)
}

func Test_HandleFilterInput_EscapeRestoresLastMatchNotPreFilter(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = "zzz"
	m.state.selected = 0
	m.state.filteredIDs = []string{}
	m.state.preFilterSelectedID = "id-api"
	m.state.lastFilteredSelectedID = "id-web"
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"], m.snapshot.Services["id-db"]}},
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}

	result, _ := m.handleFilterInput(msg)

	assert.Equal(t, 1, result.state.selected)
	assert.Empty(t, result.state.lastFilteredSelectedID)
}

func Test_HandleFilterInput_EscapeClearsFilteredState(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = "api"
	m.state.selected = 0
	m.state.filteredIDs = []string{"id-api"}
	m.state.serviceIDs = []string{"id-api", "id-web"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"]}},
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}

	result, _ := m.handleFilterInput(msg)

	assert.Empty(t, result.state.filterQuery)
	assert.False(t, result.state.filterActive)
	assert.Nil(t, result.state.filteredIDs)
	assert.Equal(t, 0, result.state.selected)
}

func Test_HandleFilterInput_EscapeRestoresSelectionPosition(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = "web"
	m.state.selected = 0
	m.state.filteredIDs = []string{"id-web"}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"], m.snapshot.Services["id-db"]}},
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}

	result, _ := m.handleFilterInput(msg)

	assert.Equal(t, 1, result.state.selected)
}

func Test_HandleFilterInput_EscapePreservesSelectionWithNoQuery(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = ""
	m.state.selected = 2
	m.state.preFilterSelectedID = "id-api"
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"], m.snapshot.Services["id-db"]}},
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}

	result, _ := m.handleFilterInput(msg)

	assert.Equal(t, 2, result.state.selected)
	assert.Empty(t, result.state.filterQuery)
	assert.False(t, result.state.filterActive)
}

func Test_HandleFilterInput_EscapeRestoresSelectionAfterZeroMatches(t *testing.T) {
	m := Model{}
	m.state.filterActive = true
	m.state.filterQuery = "zzz"
	m.state.selected = 0
	m.state.filteredIDs = []string{}
	m.state.preFilterSelectedID = "id-db"
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"], m.snapshot.Services["id-db"]}},
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}

	result, _ := m.handleFilterInput(msg)

	assert.Equal(t, 2, result.state.selected)
	assert.Empty(t, result.state.preFilterSelectedID)
}

func Test_HandleFilterInput_EscapeUnaffectedByAsideState(t *testing.T) {
	m := Model{}
	m.ui.servicesKeys = defaultKeyMap()
	m.ui.servicesViewport = viewport.New()
	m.state.asideOpen = true
	m.state.filterActive = true
	m.state.filterQuery = "web"
	m.state.serviceIDs = []string{"id-api", "id-web"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"]}},
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}

	result, _ := m.handleKeyPress(msg)

	assert.False(t, result.state.filterActive, "filter input mode should exit")
	assert.Empty(t, result.state.filterQuery, "filter query should be cleared by filter-input Escape")
}
