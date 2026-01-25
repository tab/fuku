package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/runtime"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func setupMockLogger(ctrl *gomock.Controller) *logger.MockLogger {
	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)

	mockLogger.EXPECT().WithComponent(gomock.Any()).Return(componentLogger).AnyTimes()
	componentLogger.EXPECT().Info().Return(nil).AnyTimes()
	componentLogger.EXPECT().Warn().Return(nil).AnyTimes()
	componentLogger.EXPECT().Error().Return(nil).AnyTimes()

	return mockLogger
}

func Test_NewWatcher(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	log := setupMockLogger(ctrl)
	cfg := config.DefaultConfig()
	eventBus := runtime.NewEventBus(10)

	defer eventBus.Close()

	w, err := NewWatcher(cfg, eventBus, log)
	require.NoError(t, err)
	require.NotNil(t, w)

	defer w.Close()
}

func Test_Watcher_StartsWatchingOnServiceReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	log := setupMockLogger(ctrl)
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: tmpDir,
				Watch: &config.Watch{
					Paths: []string{"*.go", "**/*.go"},
				},
			},
		},
	}

	eventBus := runtime.NewEventBus(10)
	defer eventBus.Close()

	w, err := NewWatcher(cfg, eventBus, log)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)

	eventBus.Publish(runtime.Event{
		Type: runtime.EventServiceReady,
		Data: runtime.ServiceReadyData{Service: "test-service", Tier: "default"},
	})

	time.Sleep(100 * time.Millisecond)

	m := w.(*manager)
	m.mu.RLock()
	_, exists := m.watchers["test-service"]
	m.mu.RUnlock()

	assert.True(t, exists, "watcher should be registered for service")
}

func Test_Watcher_StopsWatchingOnServiceStopped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	log := setupMockLogger(ctrl)
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: tmpDir,
				Watch: &config.Watch{
					Paths: []string{"*.go"},
				},
			},
		},
	}

	eventBus := runtime.NewEventBus(10)
	defer eventBus.Close()

	w, err := NewWatcher(cfg, eventBus, log)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)

	eventBus.Publish(runtime.Event{
		Type: runtime.EventServiceReady,
		Data: runtime.ServiceReadyData{Service: "test-service", Tier: "default"},
	})

	time.Sleep(100 * time.Millisecond)

	eventBus.Publish(runtime.Event{
		Type: runtime.EventServiceStopped,
		Data: runtime.ServiceStoppedData{Service: "test-service", Tier: "default"},
	})

	time.Sleep(100 * time.Millisecond)

	m := w.(*manager)
	m.mu.RLock()
	_, exists := m.watchers["test-service"]
	m.mu.RUnlock()

	assert.False(t, exists, "watcher should be removed after service stopped")
}

func Test_Watcher_PublishesEventOnFileChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	log := setupMockLogger(ctrl)
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "main.go")
	err := os.WriteFile(testFile, []byte("package main"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: tmpDir,
				Watch: &config.Watch{
					Paths: []string{"*.go", "**/*.go"},
				},
			},
		},
	}

	eventBus := runtime.NewEventBus(10)
	defer eventBus.Close()

	w, err := NewWatcher(cfg, eventBus, log)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := eventBus.Subscribe(ctx)

	w.Start(ctx)

	eventBus.Publish(runtime.Event{
		Type: runtime.EventServiceReady,
		Data: runtime.ServiceReadyData{Service: "test-service", Tier: "default"},
	})

	time.Sleep(200 * time.Millisecond)

	err = os.WriteFile(testFile, []byte("package main\n// modified"), 0644)
	require.NoError(t, err)

	var watchTriggered bool

	timeout := time.After(3 * time.Second)

	for !watchTriggered {
		select {
		case event := <-eventCh:
			if event.Type == runtime.EventWatchTriggered {
				data, ok := event.Data.(runtime.WatchTriggeredData)
				assert.True(t, ok)
				assert.Equal(t, "test-service", data.Service)
				assert.NotEmpty(t, data.ChangedFiles)

				watchTriggered = true
			}
		case <-timeout:
			t.Fatal("timeout waiting for watch event")
		}
	}
}

func Test_Watcher_IgnoresTestFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	log := setupMockLogger(ctrl)
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "main_test.go")
	err := os.WriteFile(testFile, []byte("package main"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: tmpDir,
				Watch: &config.Watch{
					Paths:  []string{"*.go", "**/*.go"},
					Ignore: []string{"*_test.go"},
				},
			},
		},
	}

	eventBus := runtime.NewEventBus(10)
	defer eventBus.Close()

	w, err := NewWatcher(cfg, eventBus, log)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := eventBus.Subscribe(ctx)

	w.Start(ctx)

	eventBus.Publish(runtime.Event{
		Type: runtime.EventServiceReady,
		Data: runtime.ServiceReadyData{Service: "test-service", Tier: "default"},
	})

	time.Sleep(100 * time.Millisecond)

	err = os.WriteFile(testFile, []byte("package main\n// modified"), 0644)
	require.NoError(t, err)

	timeout := time.After(700 * time.Millisecond)

	for {
		select {
		case event := <-eventCh:
			if event.Type == runtime.EventWatchTriggered {
				t.Fatal("should not receive event for ignored file")
			}
		case <-timeout:
			return
		}
	}
}

func Test_Watcher_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	log := setupMockLogger(ctrl)
	cfg := config.DefaultConfig()
	eventBus := runtime.NewEventBus(10)

	defer eventBus.Close()

	w, err := NewWatcher(cfg, eventBus, log)
	require.NoError(t, err)

	w.Close()
	w.Close()
}

func Test_Watcher_SkipsServiceWithoutWatchConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	log := setupMockLogger(ctrl)
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"no-watch-service": {
				Dir:   tmpDir,
				Watch: nil,
			},
		},
	}

	eventBus := runtime.NewEventBus(10)
	defer eventBus.Close()

	w, err := NewWatcher(cfg, eventBus, log)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)

	eventBus.Publish(runtime.Event{
		Type: runtime.EventServiceReady,
		Data: runtime.ServiceReadyData{Service: "no-watch-service", Tier: "default"},
	})

	time.Sleep(100 * time.Millisecond)

	m := w.(*manager)
	m.mu.RLock()
	_, exists := m.watchers["no-watch-service"]
	m.mu.RUnlock()

	assert.False(t, exists, "watcher should not be registered for service without watch config")
}

func Test_shouldSkipDir(t *testing.T) {
	tests := []struct {
		name   string
		dir    string
		expect bool
	}{
		{
			name:   "git directory",
			dir:    ".git",
			expect: true,
		},
		{
			name:   "node_modules",
			dir:    "node_modules",
			expect: true,
		},
		{
			name:   "vendor",
			dir:    "vendor",
			expect: true,
		},
		{
			name:   "idea",
			dir:    ".idea",
			expect: true,
		},
		{
			name:   "vscode",
			dir:    ".vscode",
			expect: true,
		},
		{
			name:   "src directory",
			dir:    "src",
			expect: false,
		},
		{
			name:   "internal directory",
			dir:    "internal",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipDir(tt.dir)
			assert.Equal(t, tt.expect, result)
		})
	}
}
