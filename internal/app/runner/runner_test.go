package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func Test_Run_ProfileNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	cfg := &config.Config{
		Profiles: map[string]interface{}{},
	}

	r := &runner{
		cfg: cfg,
		log: mockLogger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := r.Run(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "profile nonexistent not found")
}

func Test_Run_DependencyResolutionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	cfg := &config.Config{
		Profiles: map[string]interface{}{
			"test": []interface{}{"service1"},
		},
		Services: map[string]*config.Service{
			"service1": {Dir: "service1", DependsOn: []string{"service2"}},
			"service2": {Dir: "service2", DependsOn: []string{"service1"}},
		},
	}

	r := &runner{
		cfg: cfg,
		log: mockLogger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := r.Run(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve service dependencies")
	assert.Contains(t, err.Error(), "circular dependency detected")
}

func Test_Run_ServiceNotFoundInProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	cfg := &config.Config{
		Profiles: map[string]interface{}{
			"test": []interface{}{"nonexistent"},
		},
		Services: map[string]*config.Service{},
	}

	r := &runner{
		cfg: cfg,
		log: mockLogger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := r.Run(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve service dependencies")
	assert.Contains(t, err.Error(), "service 'nonexistent' not found")
}

func Test_StartService_RelativePath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	tmpDir, err := os.MkdirTemp("", "fuku_test_")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	serviceDir := filepath.Join(tmpDir, "testservice")
	err = os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	makefile := filepath.Join(serviceDir, "Makefile")
	err = os.WriteFile(makefile, []byte("run:\n\techo \"service started\"\n"), 0644)
	require.NoError(t, err)

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	r := &runner{
		log: mockLogger,
	}

	service := &config.Service{
		Dir: "testservice",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	process, err := r.startService(ctx, "test", service)
	require.NoError(t, err)
	require.NotNil(t, process)

	select {
	case <-process.done:
	case <-time.After(3 * time.Second):
		t.Fatal("service didn't complete in time")
	}
}

func Test_StartService_AbsolutePath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	tmpDir, err := os.MkdirTemp("", "fuku_test_")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	makefile := filepath.Join(tmpDir, "Makefile")
	err = os.WriteFile(makefile, []byte("run:\n\techo \"service started\"\n"), 0644)
	require.NoError(t, err)

	r := &runner{
		log: mockLogger,
	}

	service := &config.Service{
		Dir: tmpDir,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	process, err := r.startService(ctx, "test", service)
	require.NoError(t, err)
	require.NotNil(t, process)

	select {
	case <-process.done:
	case <-time.After(3 * time.Second):
		t.Fatal("service didn't complete in time")
	}
}

func Test_StartService_DirectoryNotExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	r := &runner{
		log: mockLogger,
	}

	service := &config.Service{
		Dir: "/nonexistent/directory",
	}

	ctx := context.Background()
	process, err := r.startService(ctx, "test", service)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service directory does not exist")
	assert.Nil(t, process)
}

func Test_StartService_WithEnvFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	tmpDir, err := os.MkdirTemp("", "fuku_test_")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	makefile := filepath.Join(tmpDir, "Makefile")
	err = os.WriteFile(makefile, []byte("run:\n\techo \"service started\"\n"), 0644)
	require.NoError(t, err)

	envFile := filepath.Join(tmpDir, ".env.development")
	err = os.WriteFile(envFile, []byte("TEST_VAR=test_value\n"), 0644)
	require.NoError(t, err)

	r := &runner{
		log: mockLogger,
	}

	service := &config.Service{
		Dir: tmpDir,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	process, err := r.startService(ctx, "test", service)
	require.NoError(t, err)
	require.NotNil(t, process)

	select {
	case <-process.done:
	case <-time.After(3 * time.Second):
		t.Fatal("service didn't complete in time")
	}
}

func Test_StartService_StdoutPipeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()

	tmpDir, err := os.MkdirTemp("", "fuku_test_")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	err = os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte("run:\n\tfalse\n"), 0644)
	require.NoError(t, err)

	r := &runner{
		log: mockLogger,
	}

	service := &config.Service{
		Dir: tmpDir,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	process, err := r.startService(ctx, "test", service)

	assert.Error(t, err)
	assert.Nil(t, process)
}

func Test_StreamLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	r := &runner{
		log: mockLogger,
	}

	testInput := "line1\nline2\nline3\n"
	reader := strings.NewReader(testInput)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		r.streamLogs("testservice", reader, "STDOUT")
	}()

	wg.Wait()
}

func Test_StreamLogs_WithError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	r := &runner{
		log: mockLogger,
	}

	pr, pw := io.Pipe()
	pw.CloseWithError(fmt.Errorf("test error"))

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		r.streamLogs("testservice", pr, "STDERR")
	}()

	wg.Wait()
}

func Test_StopAllProcesses_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	r := &runner{
		log: mockLogger,
	}

	processes := []*serviceProcess{}
	r.stopAllProcesses(processes)
}

func Test_StopAllProcesses_NilProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	r := &runner{
		log: mockLogger,
	}

	cmdWithNilProcess := exec.Command("echo", "test")
	processes := []*serviceProcess{
		{name: "test", cmd: cmdWithNilProcess},
	}

	require.NotPanics(t, func() {
		r.stopAllProcesses(processes)
	})
}

func Test_StopAllProcesses_SignalError(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("skipping process signal test in CI environment")
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()

	r := &runner{
		log: mockLogger,
	}

	tmpDir, err := os.MkdirTemp("", "fuku_test_")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	makefile := filepath.Join(tmpDir, "Makefile")
	err = os.WriteFile(makefile, []byte("run:\n\tsleep 60\n"), 0644)
	require.NoError(t, err)

	service := &config.Service{Dir: tmpDir}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	process, err := r.startService(ctx, "test", service)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	if process.cmd.Process != nil {
		_ = process.cmd.Process.Kill()
	}

	processes := []*serviceProcess{process}
	r.stopAllProcesses(processes)
}

func Test_Run_SuccessfulExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	tmpDir, err := os.MkdirTemp("", "fuku_test_")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	makefile := filepath.Join(tmpDir, "Makefile")
	err = os.WriteFile(makefile, []byte("run:\n\techo \"service started\"\n"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Profiles: map[string]interface{}{
			"test": []interface{}{"service1"},
		},
		Services: map[string]*config.Service{
			"service1": {Dir: tmpDir, DependsOn: []string{}},
		},
	}

	r := &runner{
		cfg: cfg,
		log: mockLogger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	err = r.Run(ctx, "test")
	assert.NoError(t, err)
}

func Test_Run_StartServiceFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()

	cfg := &config.Config{
		Profiles: map[string]interface{}{
			"test": []interface{}{"service1"},
		},
		Services: map[string]*config.Service{
			"service1": {Dir: "/nonexistent/directory", DependsOn: []string{}},
		},
	}

	r := &runner{
		cfg: cfg,
		log: mockLogger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := r.Run(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start service 'service1'")
}

func Test_StartService_GetWorkingDirectoryError(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("skipping working directory test in CI environment")
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	r := &runner{
		log: mockLogger,
	}

	service := &config.Service{
		Dir: "relative/path",
	}

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()

	tempDir, err := os.MkdirTemp("", "fuku_test_")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	err = os.Chmod(tempDir, 0000)
	require.NoError(t, err)
	defer func() { _ = os.Chmod(tempDir, 0755) }()

	ctx := context.Background()
	process, err := r.startService(ctx, "test", service)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get working directory")
	assert.Nil(t, process)

	_ = os.Chmod(tempDir, 0755)
}

func Test_ResolveProfileServices(t *testing.T) {
	tests := []struct {
		name         string
		services     map[string]*config.Service
		profiles     map[string]interface{}
		profileName  string
		expected     []string
		expectError  bool
		errorMessage string
	}{
		{
			name: "Wildcard profile includes all services",
			services: map[string]*config.Service{
				"api":      {Dir: "api"},
				"frontend": {Dir: "frontend"},
			},
			profiles:    map[string]interface{}{"all": "*"},
			profileName: "all",
			expected:    []string{"api", "frontend"},
		},
		{
			name: "Specific services in profile",
			services: map[string]*config.Service{
				"api":      {Dir: "api"},
				"frontend": {Dir: "frontend"},
			},
			profiles:    map[string]interface{}{"dev": []interface{}{"api"}},
			profileName: "dev",
			expected:    []string{"api"},
		},
		{
			name: "Profile not found",
			services: map[string]*config.Service{
				"api": {Dir: "api"},
			},
			profiles:     map[string]interface{}{},
			profileName:  "nonexistent",
			expectError:  true,
			errorMessage: "profile nonexistent not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Services: tt.services,
				Profiles: tt.profiles,
			}

			result, err := cfg.GetServicesForProfile(tt.profileName)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}

func Test_Run_WithProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()
	mockLogger.EXPECT().Error().Return(nil).AnyTimes()

	tmpDir, err := os.MkdirTemp("", "fuku_test_")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	makefile := filepath.Join(tmpDir, "Makefile")
	err = os.WriteFile(makefile, []byte("run:\n\techo \"service started\"\n"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Profiles: map[string]interface{}{
			"test": []interface{}{"service1"},
		},
		Services: map[string]*config.Service{
			"service1": {Dir: tmpDir, DependsOn: []string{}},
		},
	}

	r := &runner{
		cfg: cfg,
		log: mockLogger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	err = r.Run(ctx, "test")
	assert.NoError(t, err)
}
