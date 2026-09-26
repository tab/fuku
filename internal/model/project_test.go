package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Project_Service(t *testing.T) {
	api := Service{Name: "api", Directory: "services/api", Tier: "edge"}
	db := Service{Name: "db", Directory: "services/db", Tier: "foundation"}
	project := Project{
		Services: []Service{db, api},
	}

	tests := []struct {
		name     string
		project  Project
		lookup   string
		expected Service
		exists   bool
	}{
		{
			name:     "declared service",
			project:  project,
			lookup:   "api",
			expected: api,
			exists:   true,
		},
		{
			name:     "unknown service",
			project:  project,
			lookup:   "web",
			expected: Service{},
			exists:   false,
		},
		{
			name:     "empty project",
			project:  Project{},
			lookup:   "api",
			expected: Service{},
			exists:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, exists := tt.project.Service(tt.lookup)

			assert.Equal(t, tt.expected, service)
			assert.Equal(t, tt.exists, exists)
		})
	}
}
