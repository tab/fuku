package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/results"
	"fuku/internal/app/ui"
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


func Test_BuildDependencyLayers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	tests := []struct {
		name        string
		services    map[string]*config.Service
		input       []string
		expectError bool
		expectedLen int
	}{
		{
			name: "Single layer - no dependencies",
			services: map[string]*config.Service{
				"service1": {Dir: "dir1"},
				"service2": {Dir: "dir2"},
			},
			input:       []string{"service1", "service2"},
			expectedLen: 1,
		},
		{
			name: "Two layers",
			services: map[string]*config.Service{
				"service1": {Dir: "dir1"},
				"service2": {Dir: "dir2", DependsOn: []string{"service1"}},
			},
			input:       []string{"service1", "service2"},
			expectedLen: 2,
		},
		{
			name: "Three layers",
			services: map[string]*config.Service{
				"service1": {Dir: "dir1"},
				"service2": {Dir: "dir2", DependsOn: []string{"service1"}},
				"service3": {Dir: "dir3", DependsOn: []string{"service2"}},
			},
			input:       []string{"service1", "service2", "service3"},
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runner{
				cfg: &config.Config{Services: tt.services},
				log: mockLogger,
			}

			layers, err := r.buildDependencyLayers(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, layers, tt.expectedLen)

				for i, layer := range layers {
					assert.Equal(t, i, layer.level)
					assert.NotEmpty(t, layer.services)
				}
			}
		})
	}
}

func Test_Runner_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	tests := []struct {
		name        string
		profile     string
		services    map[string]*config.Service
		profiles    map[string]interface{}
		expectError bool
		errorMsg    string
		before      func()
	}{
		{
			name:    "Profile not found",
			profile: "nonexistent",
			services: map[string]*config.Service{
				"service1": {Dir: "dir1"},
			},
			profiles:    map[string]interface{}{},
			expectError: true,
			errorMsg:    "failed to resolve profile services",
			before:      func() {},
		},
		{
			name:    "Service not found in profile",
			profile: "test",
			services: map[string]*config.Service{
				"service1": {Dir: "dir1"},
			},
			profiles: map[string]interface{}{
				"test": []interface{}{"nonexistent"},
			},
			expectError: true,
			errorMsg:    "failed to resolve service dependencies",
			before:      func() {},
		},
		{
			name:    "Circular dependency error",
			profile: "test",
			services: map[string]*config.Service{
				"service1": {Dir: "dir1", DependsOn: []string{"service2"}},
				"service2": {Dir: "dir2", DependsOn: []string{"service1"}},
			},
			profiles: map[string]interface{}{
				"test": []interface{}{"service1", "service2"},
			},
			expectError: true,
			errorMsg:    "failed to resolve service dependencies",
			before:      func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			cfg := &config.Config{
				Services: tt.services,
				Profiles: tt.profiles,
			}

			r := &runner{
				cfg: cfg,
				log: mockLogger,
			}

			ctx := context.Background()
			err := r.Run(ctx, tt.profile)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_BuildDependencyLayers_ErrorCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	tests := []struct {
		name        string
		services    map[string]*config.Service
		input       []string
		expectError bool
		errorMsg    string
	}{
		{
			name: "Service not found",
			services: map[string]*config.Service{
				"service1": {Dir: "dir1"},
			},
			input:       []string{"service1", "missing"},
			expectError: true,
			errorMsg:    "service 'missing' not found",
		},
		{
			name: "Circular dependency",
			services: map[string]*config.Service{
				"service1": {Dir: "dir1", DependsOn: []string{"service2"}},
				"service2": {Dir: "dir2", DependsOn: []string{"service1"}},
			},
			input:       []string{"service1", "service2"},
			expectError: true,
			errorMsg:    "circular dependency detected among services",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runner{
				cfg: &config.Config{Services: tt.services},
				log: mockLogger,
			}

			layers, err := r.buildDependencyLayers(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, layers)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, layers)
			}
		})
	}
}

func Test_StartSingleService_DirectoryValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	r := &runner{
		cfg: &config.Config{},
		log: mockLogger,
	}

	t.Run("Directory does not exist", func(t *testing.T) {
		service := &config.Service{Dir: "/nonexistent/directory"}
		display := ui.NewDisplay()
		process := &serviceProcess{name: "test-service"}

		err := r.startSingleService(context.Background(), "test-service", service, process, display, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service directory does not exist")
	})
}

func Test_StopAllProcesses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	tests := []struct {
		name      string
		processes map[string]*serviceProcess
		before    func()
	}{
		{
			name:      "Empty processes map",
			processes: map[string]*serviceProcess{},
			before: func() {
				// No logging expectations for empty map
			},
		},
		{
			name: "Process with nil cmd",
			processes: map[string]*serviceProcess{
				"service1": {name: "service1", cmd: nil},
			},
			before: func() {
				// No logging expected for nil cmd
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			r := &runner{
				cfg: &config.Config{},
				log: mockLogger,
			}

			display := ui.NewDisplay()

			// This should not panic and should handle nil processes gracefully
			r.stopAllProcesses(tt.processes, display)
		})
	}
}

func Test_StreamLogs_ReaderHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	r := &runner{
		cfg: &config.Config{},
		log: mockLogger,
	}

	tests := []struct {
		name       string
		input      string
		streamType string
		before     func()
	}{
		{
			name:       "STDOUT stream",
			input:      "test log line\nsecond line\n",
			streamType: "STDOUT",
			before:     func() {},
		},
		{
			name:       "STDERR stream",
			input:      "error log line\n",
			streamType: "STDERR",
			before:     func() {},
		},
		{
			name:       "Empty input",
			input:      "",
			streamType: "STDOUT",
			before:     func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			reader := strings.NewReader(tt.input)
			display := ui.NewDisplay()

			// This should process the input without errors
			r.streamLogs("test-service", reader, tt.streamType, display)

			// Test passes if no panic occurs and method completes
		})
	}
}

func Test_Runner_Run_CancellationHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	r := &runner{
		cfg: &config.Config{
			Services: map[string]*config.Service{},
			Profiles: map[string]interface{}{},
		},
		log: mockLogger,
	}

	t.Run("Context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := r.Run(ctx, "nonexistent")
		assert.Error(t, err)
	})
}

func Test_StartServicesInLayers_ErrorHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	r := &runner{
		cfg: &config.Config{
			Services: map[string]*config.Service{
				"service1": {Dir: "/nonexistent"},
			},
		},
		log: mockLogger,
	}

	t.Run("Service fails to start", func(t *testing.T) {
		layers := []serviceLayer{
			{services: []string{"service1"}, level: 0},
		}

		processes := map[string]*serviceProcess{
			"service1": {
				name:   "service1",
				result: results.NewServiceResult("service1"),
			},
		}

		display := ui.NewDisplay()
		display.AddService("service1")

		err := r.startServicesInLayers(context.Background(), layers, processes, display, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start service")
	})
}

func Test_StartSingleService_PathResolution(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	r := &runner{
		cfg: &config.Config{},
		log: mockLogger,
	}

	t.Run("Relative path gets resolved", func(t *testing.T) {
		service := &config.Service{Dir: "relative/path"}
		display := ui.NewDisplay()
		process := &serviceProcess{name: "test-service"}

		// This should fail trying to find the directory, but we test the path resolution logic
		err := r.startSingleService(context.Background(), "test-service", service, process, display, nil)

		assert.Error(t, err)
		// Should fail on directory existence, not path resolution
		assert.Contains(t, err.Error(), "service directory does not exist")
	})
}

func Test_Runner_Run_DetailedCoverage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	tests := []struct {
		name        string
		setupConfig func() *config.Config
		profile     string
		expectError bool
		errorMsg    string
	}{
		{
			name: "Invalid dependency causes error",
			setupConfig: func() *config.Config {
				return &config.Config{
					Services: map[string]*config.Service{
						"service1": {Dir: ".", DependsOn: []string{"missing"}},
					},
					Profiles: map[string]interface{}{
						"test": []interface{}{"service1"},
					},
				}
			},
			profile:     "test",
			expectError: true,
			errorMsg:    "failed to resolve service dependencies",
		},
		{
			name: "All service directory validation paths",
			setupConfig: func() *config.Config {
				return &config.Config{
					Services: map[string]*config.Service{
						"service1": {Dir: "/definitely/does/not/exist/path/12345"},
					},
					Profiles: map[string]interface{}{
						"test": []interface{}{"service1"},
					},
				}
			},
			profile:     "test",
			expectError: false, // This will fail in service startup, not validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runner{
				cfg: tt.setupConfig(),
				log: mockLogger,
			}

			ctx := context.Background()
			err := r.Run(ctx, tt.profile)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			}
		})
	}
}

func Test_Runner_Coverage_HelperMethods(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	r := &runner{
		cfg: &config.Config{
			Services: map[string]*config.Service{
				"service1": {Dir: "."},
				"service2": {Dir: ".", DependsOn: []string{"service1"}},
				"service3": {Dir: ".", DependsOn: []string{"service1", "service2"}},
			},
		},
		log: mockLogger,
	}

	t.Run("Complex dependency layers", func(t *testing.T) {
		layers, err := r.buildDependencyLayers([]string{"service1", "service2", "service3"})
		assert.NoError(t, err)
		assert.Len(t, layers, 3) // Three layers for this dependency chain

		// Verify first layer only contains service1
		assert.Equal(t, []string{"service1"}, layers[0].services)
		assert.Equal(t, 0, layers[0].level)

		// Verify second layer only contains service2
		assert.Equal(t, []string{"service2"}, layers[1].services)
		assert.Equal(t, 1, layers[1].level)

		// Verify third layer only contains service3
		assert.Equal(t, []string{"service3"}, layers[2].services)
		assert.Equal(t, 2, layers[2].level)
	})

	t.Run("Empty service list", func(t *testing.T) {
		layers, err := r.buildDependencyLayers([]string{})
		assert.NoError(t, err)
		assert.Empty(t, layers)
	})

	t.Run("Single service no dependencies", func(t *testing.T) {
		layers, err := r.buildDependencyLayers([]string{"service1"})
		assert.NoError(t, err)
		assert.Len(t, layers, 1)
		assert.Equal(t, []string{"service1"}, layers[0].services)
		assert.Equal(t, 0, layers[0].level)
	})
}

