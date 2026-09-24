package environment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/model"
)

func Test_Store_reload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockReader := NewMockReader(ctrl)

	subject := NewStore(mockSubscriber, mockReader)

	service := &model.Service{Directory: "svc/api", Environment: &model.EnvFiles{Files: []string{".env"}}}

	tests := []struct {
		name     string
		before   func()
		service  *model.Service
		expected []model.Env
	}{
		{
			name: "populates the cache keyed by id",
			before: func() {
				mockReader.EXPECT().Read("svc/api", ".env").Return([]model.Env{{Key: "APP_NAME", Value: "hub-api"}}, nil)
			},
			service:  service,
			expected: []model.Env{{Key: "APP_NAME", Value: "hub-api"}},
		},
		{
			name:     "nil service clears the cache",
			before:   func() {},
			service:  nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			subject.reload("uuid-123", tt.service)

			assert.Equal(t, tt.expected, subject.Env("uuid-123"))
		})
	}
}

func Test_Store_load(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockReader := NewMockReader(ctrl)

	subject := NewStore(mockSubscriber, mockReader)

	tests := []struct {
		name     string
		before   func()
		service  *model.Service
		expected []model.Env
	}{
		{
			name:     "nil service reads nothing",
			before:   func() {},
			service:  nil,
			expected: nil,
		},
		{
			name:     "empty directory reads nothing",
			before:   func() {},
			service:  &model.Service{},
			expected: nil,
		},
		{
			name:     "explicit empty files disables loading",
			before:   func() {},
			service:  &model.Service{Directory: "svc", Environment: &model.EnvFiles{Files: []string{}}},
			expected: nil,
		},
		{
			name: "nil environment falls back to the default files",
			before: func() {
				mockReader.EXPECT().Read("svc", ".env").Return(nil, assert.AnError)
				mockReader.EXPECT().Read("svc", ".env.local").Return(nil, assert.AnError)
				mockReader.EXPECT().Read("svc", ".env.development").Return([]model.Env{{Key: "APP_NAME", Value: "hub"}}, nil)
				mockReader.EXPECT().Read("svc", ".env.development.local").Return(nil, assert.AnError)
			},
			service:  &model.Service{Directory: "svc"},
			expected: []model.Env{{Key: "APP_NAME", Value: "hub"}},
		},
		{
			name: "configured files are read in order",
			before: func() {
				mockReader.EXPECT().Read("svc", ".env.development").Return([]model.Env{{Key: "A", Value: "dev"}}, nil)
				mockReader.EXPECT().Read("svc", ".env").Return([]model.Env{{Key: "A", Value: "base"}}, nil)
			},
			service:  &model.Service{Directory: "svc", Environment: &model.EnvFiles{Files: []string{".env.development", ".env"}}},
			expected: []model.Env{{Key: "A", Value: "base"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			got := subject.load(tt.service)

			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_resolveFiles(t *testing.T) {
	tests := []struct {
		name     string
		service  *model.Service
		expected []string
	}{
		{
			name:     "nil environment uses the defaults",
			service:  &model.Service{},
			expected: []string{".env", ".env.local", ".env.development", ".env.development.local"},
		},
		{
			name:     "nil files uses the defaults",
			service:  &model.Service{Environment: &model.EnvFiles{}},
			expected: []string{".env", ".env.local", ".env.development", ".env.development.local"},
		},
		{
			name:     "explicit empty files keeps the empty list",
			service:  &model.Service{Environment: &model.EnvFiles{Files: []string{}}},
			expected: []string{},
		},
		{
			name:     "configured files are kept in order",
			service:  &model.Service{Environment: &model.EnvFiles{Files: []string{".env.development", ".env"}}},
			expected: []string{".env.development", ".env"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, resolveFiles(tt.service))
		})
	}
}
