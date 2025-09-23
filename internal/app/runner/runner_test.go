package runner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewRunner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	cfg := &config.Config{}

	r := NewRunner(cfg, mockLogger)
	assert.NotNil(t, r)

	instance, ok := r.(*runner)
	assert.True(t, ok)
	assert.Equal(t, cfg, instance.cfg)
	assert.Equal(t, mockLogger, instance.log)
}

func Test_ResolveServiceOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	tests := []struct {
		name         string
		services     map[string]*config.Service
		input        []string
		expected     []string
		expectError  bool
		errorMessage string
	}{
		{
			name: "Simple dependency chain",
			services: map[string]*config.Service{
				"api":          {Dir: "api", DependsOn: []string{"auth"}},
				"auth":         {Dir: "auth", DependsOn: []string{}},
				"frontend-api": {Dir: "frontend", DependsOn: []string{"api"}},
			},
			input:    []string{"frontend-api", "api"},
			expected: []string{"auth", "api", "frontend-api"},
		},
		{
			name: "No dependencies",
			services: map[string]*config.Service{
				"service1": {Dir: "service1", DependsOn: []string{}},
				"service2": {Dir: "service2", DependsOn: []string{}},
			},
			input:    []string{"service1", "service2"},
			expected: []string{"service1", "service2"},
		},
		{
			name: "Circular dependency",
			services: map[string]*config.Service{
				"service1": {Dir: "service1", DependsOn: []string{"service2"}},
				"service2": {Dir: "service2", DependsOn: []string{"service1"}},
			},
			input:        []string{"service1"},
			expectError:  true,
			errorMessage: "circular dependency detected for service 'service1'",
		},
		{
			name: "Service not found",
			services: map[string]*config.Service{
				"service1": {Dir: "service1", DependsOn: []string{}},
			},
			input:        []string{"nonexistent"},
			expectError:  true,
			errorMessage: "service 'nonexistent' not found",
		},
		{
			name: "Complex dependency tree",
			services: map[string]*config.Service{
				"api":             {Dir: "api", DependsOn: []string{"auth", "account"}},
				"frontend-api":    {Dir: "frontend", DependsOn: []string{"api"}},
				"auth":            {Dir: "auth", DependsOn: []string{}},
				"account":         {Dir: "account", DependsOn: []string{}},
				"file-management": {Dir: "file", DependsOn: []string{}},
			},
			input:    []string{"frontend-api", "file-management"},
			expected: []string{"auth", "account", "api", "frontend-api", "file-management"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Services: tt.services,
			}

			r := &runner{
				cfg: cfg,
				log: mockLogger,
			}

			result, err := r.resolveServiceOrder(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func Test_Run_ScopeNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	cfg := &config.Config{
		Scopes: map[string]*config.Scope{},
	}

	r := &runner{
		cfg: cfg,
		log: mockLogger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := r.Run(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scope 'nonexistent' not found")
}
