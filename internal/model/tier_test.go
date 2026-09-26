package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Tiers_Services(t *testing.T) {
	api := &Service{Name: "api"}
	auth := &Service{Name: "auth"}
	web := &Service{Name: "web"}

	tests := []struct {
		name     string
		tiers    Tiers
		expected []*Service
	}{
		{
			name:     "no tiers",
			tiers:    nil,
			expected: nil,
		},
		{
			name:     "tiers without services",
			tiers:    Tiers{{Name: "platform"}, {Name: "edge"}},
			expected: nil,
		},
		{
			name:     "every tier in startup order",
			tiers:    Tiers{{Name: "platform", Services: []*Service{api, auth}}, {Name: "edge"}, {Name: "sidekick", Services: []*Service{web}}},
			expected: []*Service{api, auth, web},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tiers.Services()

			assert.Equal(t, tt.expected, result)
		})
	}
}
