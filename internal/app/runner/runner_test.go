package runner

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/ui"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewRunner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockDisplay := ui.NewMockDisplay(ctrl)
	cfg := &config.Config{}

	r := NewRunner(cfg, mockDisplay, mockLogger)
	assert.NotNil(t, r)

	instance, ok := r.(*runner)
	assert.True(t, ok)
	assert.Equal(t, cfg, instance.cfg)
	assert.Equal(t, mockDisplay, instance.display)
	assert.Equal(t, mockLogger, instance.log)
}

func Test_Run(t *testing.T) {
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Services: tt.services,
				Profiles: tt.profiles,
			}

			mockDisplay := ui.NewMockDisplay(ctrl)

			// Set up mock expectations for Display methods
			mockDisplay.EXPECT().SetProgress(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockDisplay.EXPECT().Phase(gomock.Any()).AnyTimes()
			mockDisplay.EXPECT().Add(gomock.Any()).AnyTimes()
			mockDisplay.EXPECT().Update(gomock.Any(), gomock.Any()).AnyTimes()
			mockDisplay.EXPECT().IsBootstrap().Return(true).AnyTimes()
			mockDisplay.EXPECT().ShowSummary().AnyTimes()
			mockDisplay.EXPECT().IsReady().Return(false).AnyTimes()

			r := &runner{
				cfg:     cfg,
				display: mockDisplay,
				log:     mockLogger,
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

func Test_Run_CancellationHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	mockDisplay := ui.NewMockDisplay(ctrl)

	// Set up mock expectations for Display methods
	mockDisplay.EXPECT().SetProgress(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockDisplay.EXPECT().Phase(gomock.Any()).AnyTimes()

	r := &runner{
		cfg: &config.Config{
			Services: map[string]*config.Service{},
			Profiles: map[string]interface{}{},
		},
		display: mockDisplay,
		log:     mockLogger,
	}

	t.Run("Context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := r.Run(ctx, "nonexistent")
		assert.Error(t, err)
	})
}

func Test_Run_ErrorPaths(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	tests := []struct {
		name    string
		config  *config.Config
		profile string
		wantErr bool
	}{
		{
			name: "Nonexistent profile",
			config: &config.Config{
				Services: map[string]*config.Service{},
				Profiles: map[string]interface{}{},
			},
			profile: "nonexistent",
			wantErr: true,
		},
		{
			name: "Profile with nonexistent service",
			config: &config.Config{
				Services: map[string]*config.Service{},
				Profiles: map[string]interface{}{
					"test": map[string]interface{}{"include": []string{"nonexistent-service"}},
				},
			},
			profile: "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDisplay := ui.NewMockDisplay(ctrl)

			// Set up mock expectations for Display methods
			mockDisplay.EXPECT().SetProgress(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockDisplay.EXPECT().Phase(gomock.Any()).AnyTimes()

			r := &runner{
				cfg:     tt.config,
				display: mockDisplay,
				log:     mockLogger,
			}

			err := r.Run(context.Background(), tt.profile)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_startServicesInLayers_ErrorHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	mockDisplay := ui.NewMockDisplay(ctrl)

	// Set up mock expectations for Display methods
	mockDisplay.EXPECT().SetProgress(1, 1, gomock.Any())
	mockDisplay.EXPECT().ShowLayer(0, []string{"service1"})
	mockDisplay.EXPECT().Update("service1", gomock.Any()).AnyTimes()
	mockDisplay.EXPECT().UpdateLayer(0, []string{"service1"}).AnyTimes()
	mockDisplay.EXPECT().IsBootstrap().Return(true).AnyTimes()
	mockDisplay.EXPECT().Error("service1", gomock.Any())

	r := &runner{
		cfg: &config.Config{
			Services: map[string]*config.Service{
				"service1": {Dir: "/nonexistent"},
			},
		},
		display: mockDisplay,
		log:     mockLogger,
	}

	t.Run("Service fails to start", func(t *testing.T) {
		layers := []serviceLayer{
			{services: []string{"service1"}, level: 0},
		}

		processes := map[string]*serviceProcess{
			"service1": {
				name: "service1",
			},
		}

		err := r.startServicesInLayers(context.Background(), layers, processes, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start service")
	})
}

func Test_startSingleService_PathResolution(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	mockDisplay := ui.NewMockDisplay(ctrl)
	r := &runner{
		cfg:     &config.Config{},
		display: mockDisplay,
		log:     mockLogger,
	}

	t.Run("Relative path gets resolved", func(t *testing.T) {
		service := &config.Service{Dir: "relative/path"}
		process := &serviceProcess{name: "test-service"}

		err := r.startSingleService(context.Background(), "test-service", service, process, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service directory does not exist")
	})
}

func Test_startSingleService_DirectoryValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	mockDisplay := ui.NewMockDisplay(ctrl)
	r := &runner{
		cfg:     &config.Config{},
		display: mockDisplay,
		log:     mockLogger,
	}

	t.Run("Directory does not exist", func(t *testing.T) {
		service := &config.Service{Dir: "/nonexistent/directory"}
		process := &serviceProcess{name: "test-service"}

		err := r.startSingleService(context.Background(), "test-service", service, process, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service directory does not exist")
	})
}

func Test_streamLogs_ReaderHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	mockDisplay := ui.NewMockDisplay(ctrl)

	// Set up mock expectations for Display methods
	mockDisplay.EXPECT().BufferLog(gomock.Any()).AnyTimes()
	mockDisplay.EXPECT().IsBootstrap().Return(true).AnyTimes()

	r := &runner{
		cfg:     &config.Config{},
		display: mockDisplay,
		log:     mockLogger,
	}

	tests := []struct {
		name       string
		input      string
		streamType string
	}{
		{
			name:       "STDOUT stream",
			input:      "test log line\nsecond line\n",
			streamType: "STDOUT",
		},
		{
			name:       "STDERR stream",
			input:      "error log line\n",
			streamType: "STDERR",
		},
		{
			name:       "Empty input",
			input:      "",
			streamType: "STDOUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)

			r.streamLogs("test-service", reader, tt.streamType)
		})
	}
}

func Test_stopAllProcesses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	mockDisplay := ui.NewMockDisplay(ctrl)
	r := &runner{
		cfg:     &config.Config{},
		display: mockDisplay,
		log:     mockLogger,
	}

	tests := []struct {
		name      string
		processes map[string]*serviceProcess
	}{
		{
			name:      "Empty processes map",
			processes: map[string]*serviceProcess{},
		},
		{
			name: "Process with nil cmd",
			processes: map[string]*serviceProcess{
				"service1": {name: "service1", cmd: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.stopAllProcesses(tt.processes)
		})
	}
}

func Test_stopAllProcesses_EdgeCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockEvent := logger.NewMockEvent(ctrl)
	mockLogger.EXPECT().Debug().Return(mockEvent).AnyTimes()
	mockLogger.EXPECT().Info().Return(mockEvent).AnyTimes()
	mockLogger.EXPECT().Error().Return(mockEvent).AnyTimes()
	mockEvent.EXPECT().Msgf(gomock.Any(), gomock.Any()).AnyTimes()
	mockEvent.EXPECT().Err(gomock.Any()).Return(mockEvent).AnyTimes()

	mockDisplay := ui.NewMockDisplay(ctrl)

	// Set up mock expectations for Display methods
	mockDisplay.EXPECT().Update(gomock.Any(), gomock.Any()).AnyTimes()

	r := &runner{
		cfg:     &config.Config{},
		display: mockDisplay,
		log:     mockLogger,
	}

	t.Run("Process with running command", func(t *testing.T) {

		processes := map[string]*serviceProcess{
			"test-service": {
				name: "test-service",
				cmd:  exec.Command("sleep", "1"),
			},
		}

		err := processes["test-service"].cmd.Start()
		assert.NoError(t, err)

		r.stopAllProcesses(processes)

		processes["test-service"].cmd.Wait()
	})

	t.Run("Process already finished", func(t *testing.T) {

		processes := map[string]*serviceProcess{
			"test-service2": {
				name: "test-service2",
				cmd:  exec.Command("echo", "done"),
			},
		}

		err := processes["test-service2"].cmd.Run()
		assert.NoError(t, err)

		r.stopAllProcesses(processes)
	})
}

func Test_buildDependencyLayers(t *testing.T) {
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
			mockDisplay := ui.NewMockDisplay(ctrl)
			r := &runner{
				cfg:     &config.Config{Services: tt.services},
				display: mockDisplay,
				log:     mockLogger,
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

func Test_buildDependencyLayers_ErrorCases(t *testing.T) {
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
			mockDisplay := ui.NewMockDisplay(ctrl)
			r := &runner{
				cfg:     &config.Config{Services: tt.services},
				display: mockDisplay,
				log:     mockLogger,
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
