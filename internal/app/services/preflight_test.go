package services

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_NewCleaner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProfiles := NewMockProfileResolver(ctrl)
	mockPreflight := NewMockPreflight(ctrl)

	log := slog.New(slog.DiscardHandler)

	cleaner := NewCleaner(mockProfiles, mockPreflight, log)

	assert.NotNil(t, cleaner)
	assert.Equal(t, mockProfiles, cleaner.profiles)
	assert.Equal(t, mockPreflight, cleaner.preflight)
	assert.Equal(t, log, cleaner.log)
}

func Test_Cleaner_Cleanup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProfiles := NewMockProfileResolver(ctrl)
	mockPreflight := NewMockPreflight(ctrl)

	log := slog.New(slog.DiscardHandler)

	tiers := []model.Tier{{Name: "platform", Services: []*model.Service{{ID: "test-id-api", Name: "api", Directory: "services/api"}}}}
	dirs := map[string]string{"api": "services/api"}
	scanErr := errors.New("scan failed")

	cleaner := NewCleaner(mockProfiles, mockPreflight, log)

	tests := []struct {
		name     string
		before   func()
		expected error
	}{
		{
			name: "returns the profile error",
			before: func() {
				mockProfiles.EXPECT().Resolve("missing").Return(nil, contracts.ErrProfileNotFound)
			},
			expected: contracts.ErrProfileNotFound,
		},
		{
			name: "cleans nothing for a profile without services",
			before: func() {
				mockProfiles.EXPECT().Resolve("missing").Return([]model.Tier{{Name: "platform"}}, nil)
			},
		},
		{
			name: "cleans the service directories",
			before: func() {
				mockProfiles.EXPECT().Resolve("missing").Return(tiers, nil)
				mockPreflight.EXPECT().Cleanup(gomock.Any(), dirs).Return(nil)
			},
		},
		{
			name: "a failed scan is a warning",
			before: func() {
				mockProfiles.EXPECT().Resolve("missing").Return(tiers, nil)
				mockPreflight.EXPECT().Cleanup(gomock.Any(), dirs).Return(scanErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := cleaner.Cleanup(t.Context(), "missing")

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_ServiceDirs(t *testing.T) {
	services := []*model.Service{
		{Name: "api", Directory: "services/api"},
		{Name: "worker", Directory: "/opt/worker"},
	}

	dirs := serviceDirs(services)

	assert.Equal(t, map[string]string{"api": "services/api", "worker": "/opt/worker"}, dirs)
}
