package profiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_NewResolver(t *testing.T) {
	project := model.Project{Profiles: map[string]model.Profile{"all": {All: true}}}

	resolver := NewResolver(project)

	assert.Equal(t, project, resolver.project)
}

func Test_Resolver_Resolve(t *testing.T) {
	tests := []struct {
		name     string
		before   func() *Resolver
		profile  string
		expected []model.Tier
	}{
		{
			name: "wildcard keeps project order",
			before: func() *Resolver {
				return NewResolver(model.Project{
					Services: []model.Service{
						{Name: "db", Tier: "foundation"},
						{Name: "api", Tier: "platform"},
						{Name: "web", Tier: "platform"},
					},
					Profiles: map[string]model.Profile{"all": {All: true}},
				})
			},
			profile: "all",
			expected: []model.Tier{
				{Name: "foundation", Services: []*model.Service{{Name: "db", Tier: "foundation"}}},
				{Name: "platform", Services: []*model.Service{{Name: "api", Tier: "platform"}, {Name: "web", Tier: "platform"}}},
			},
		},
		{
			name: "named profile deduplicates and excludes services",
			before: func() *Resolver {
				return NewResolver(model.Project{
					Services: []model.Service{
						{Name: "db", Tier: "foundation"},
						{Name: "api", Tier: "platform"},
						{Name: "debug", Tier: "platform"},
					},
					Profiles: map[string]model.Profile{
						"backend": {Services: []string{"api", "debug", "db", "api"}},
					},
					Exclude: []string{"debug"},
				})
			},
			profile: "backend",
			expected: []model.Tier{
				{Name: "foundation", Services: []*model.Service{{Name: "db", Tier: "foundation"}}},
				{Name: "platform", Services: []*model.Service{{Name: "api", Tier: "platform"}}},
			},
		},
		{
			name: "empty profile returns an empty non-nil result",
			before: func() *Resolver {
				return NewResolver(model.Project{
					Services: []model.Service{},
					Profiles: map[string]model.Profile{"empty": {}},
				})
			},
			profile:  "empty",
			expected: []model.Tier{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := tt.before()

			actual, err := resolver.Resolve(tt.profile)

			require.NoError(t, err)
			require.NotNil(t, actual)
			require.Len(t, actual, len(tt.expected))

			for i, tier := range tt.expected {
				assert.Equal(t, tier.Name, actual[i].Name)
				assert.Equal(t, tier.Services, actual[i].Services)
			}
		})
	}
}

func Test_Resolver_Resolve_PreservesServices(t *testing.T) {
	project := model.Project{
		Services: []model.Service{
			{ID: "test-id-db", Name: "db", Tier: "foundation", Directory: "db", Command: "make run"},
			{ID: "test-id-api", Name: "api", Tier: "platform", Readiness: &model.Readiness{Type: model.ReadinessTCP, Address: ":8080"}},
			{ID: "test-id-web", Name: "web", Tier: "platform", Watch: &model.Watch{Include: []string{"**/*.go"}}, Environment: &model.EnvFiles{Files: []string{}}},
		},
		Profiles: map[string]model.Profile{"all": {All: true}},
	}
	resolver := NewResolver(project)

	foundation := []*model.Service{&project.Services[0]}
	platform := []*model.Service{&project.Services[1], &project.Services[2]}

	tiers, err := resolver.Resolve("all")

	require.NoError(t, err)
	require.Len(t, tiers, 2)
	assert.NotEmpty(t, tiers[0].ID)
	assert.NotEmpty(t, tiers[1].ID)
	assert.NotEqual(t, tiers[0].ID, tiers[1].ID)
	assert.Equal(t, foundation, tiers[0].Services)
	assert.Equal(t, platform, tiers[1].Services)
	assert.NotSame(t, foundation[0], tiers[0].Services[0])
}

func Test_Resolver_Resolve_RepeatsTheSameServices(t *testing.T) {
	project := model.Project{
		Services: []model.Service{
			{ID: "test-id-db", Name: "db", Tier: "foundation", Directory: "db", Command: "make run"},
			{ID: "test-id-api", Name: "api", Tier: "platform", Readiness: &model.Readiness{Type: model.ReadinessTCP, Address: ":8080"}},
		},
		Profiles: map[string]model.Profile{"all": {All: true}},
	}
	resolver := NewResolver(project)

	first, err := resolver.Resolve("all")
	require.NoError(t, err)

	again, err := resolver.Resolve("all")

	require.NoError(t, err)
	require.Len(t, again, 2)
	assert.Equal(t, first[0].Services, again[0].Services)
	assert.Equal(t, first[1].Services, again[1].Services)
}

func Test_Resolver_Resolve_Errors(t *testing.T) {
	tests := []struct {
		name          string
		before        func() *Resolver
		profile       string
		expectedError error
	}{
		{
			name: "profile not found",
			before: func() *Resolver {
				return NewResolver(model.Project{
					Services: []model.Service{},
					Profiles: map[string]model.Profile{},
				})
			},
			profile:       "missing",
			expectedError: contracts.ErrProfileNotFound,
		},
		{
			name: "service not found",
			before: func() *Resolver {
				return NewResolver(model.Project{
					Services: []model.Service{},
					Profiles: map[string]model.Profile{
						"broken": {Services: []string{"missing"}},
					},
				})
			},
			profile:       "broken",
			expectedError: contracts.ErrServiceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := tt.before()

			_, err := resolver.Resolve(tt.profile)

			assert.ErrorIs(t, err, tt.expectedError)
		})
	}
}

func Test_Resolver_Resolve_ExcludedMissingService(t *testing.T) {
	project := model.Project{
		Services: []model.Service{},
		Profiles: map[string]model.Profile{
			"clean": {Services: []string{"missing"}},
		},
		Exclude: []string{"missing"},
	}
	resolver := NewResolver(project)

	tiers, err := resolver.Resolve("clean")

	require.NoError(t, err)
	assert.Empty(t, tiers)
}
