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

	"fuku/internal/app/bus"
	"fuku/internal/app/logs"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewWatcher(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent(gomock.Any()).Return(componentLog).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	w, err := NewWatcher(config.DefaultConfig(), b, mockLog)
	require.NoError(t, err)
	require.NotNil(t, w)

	defer w.Close()
}

func Test_Watcher_StartsWatchingOnServiceReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent(gomock.Any()).Return(componentLog).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: tmpDir,
				Watch: &config.Watch{
					Include: []string{"*.go", "**/*.go"},
				},
			},
		},
	}
	cfg.Logs.Buffer = 10

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	w, err := NewWatcher(cfg, b, mockLog)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := b.Subscribe(ctx)
	w.Start(ctx)

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "test-service", Tier: "default"}},
	})

	// Wait for WatchStarted event instead of sleeping
	waitForEvent(t, eventCh, bus.EventWatchStarted)

	m := w.(*manager)
	m.mu.RLock()
	_, exists := m.watchers["test-service"]
	m.mu.RUnlock()

	assert.True(t, exists, "watcher should be registered for service")
}

func Test_Watcher_StopsWatchingOnServiceStopped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent(gomock.Any()).Return(componentLog).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: tmpDir,
				Watch: &config.Watch{
					Include: []string{"*.go"},
				},
			},
		},
	}
	cfg.Logs.Buffer = 10

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	w, err := NewWatcher(cfg, b, mockLog)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := b.Subscribe(ctx)
	w.Start(ctx)

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "test-service", Tier: "default"}},
	})

	waitForEvent(t, eventCh, bus.EventWatchStarted)

	b.Publish(bus.Message{
		Type: bus.EventServiceStopped,
		Data: bus.ServiceStopped{ServiceEvent: bus.ServiceEvent{Service: "test-service", Tier: "default"}},
	})

	waitForEvent(t, eventCh, bus.EventWatchStopped)

	m := w.(*manager)
	m.mu.RLock()
	_, exists := m.watchers["test-service"]
	m.mu.RUnlock()

	assert.False(t, exists, "watcher should be removed after service stopped")
}

func Test_Watcher_PublishesEventOnFileChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent(gomock.Any()).Return(componentLog).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "main.go")
	err := os.WriteFile(testFile, []byte("package main"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: tmpDir,
				Watch: &config.Watch{
					Include:  []string{"*.go", "**/*.go"},
					Debounce: 10 * time.Millisecond,
				},
			},
		},
	}
	cfg.Logs.Buffer = 10

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	w, err := NewWatcher(cfg, b, mockLog)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := b.Subscribe(ctx)
	w.Start(ctx)

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "test-service", Tier: "default"}},
	})

	waitForEvent(t, eventCh, bus.EventWatchStarted)

	err = os.WriteFile(testFile, []byte("package main\n// modified"), 0644)
	require.NoError(t, err)

	timeout := time.After(3 * time.Second)

	for {
		select {
		case event := <-eventCh:
			if event.Type == bus.EventWatchTriggered {
				data, ok := event.Data.(bus.WatchTriggered)
				assert.True(t, ok)
				assert.Equal(t, "test-service", data.Service)
				assert.NotEmpty(t, data.ChangedFiles)

				return
			}
		case <-timeout:
			t.Fatal("timeout waiting for watch event")
		}
	}
}

func Test_Watcher_IgnoresTestFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent(gomock.Any()).Return(componentLog).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "main_test.go")
	err := os.WriteFile(testFile, []byte("package main"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: tmpDir,
				Watch: &config.Watch{
					Include:  []string{"*.go", "**/*.go"},
					Ignore:   []string{"*_test.go"},
					Debounce: 10 * time.Millisecond,
				},
			},
		},
	}
	cfg.Logs.Buffer = 10

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	w, err := NewWatcher(cfg, b, mockLog)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := b.Subscribe(ctx)
	w.Start(ctx)

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "test-service", Tier: "default"}},
	})

	waitForEvent(t, eventCh, bus.EventWatchStarted)

	err = os.WriteFile(testFile, []byte("package main\n// modified"), 0644)
	require.NoError(t, err)

	// Wait briefly and verify no WatchTriggered event
	timeout := time.After(200 * time.Millisecond)

	for {
		select {
		case event := <-eventCh:
			if event.Type == bus.EventWatchTriggered {
				t.Fatal("should not receive event for ignored file")
			}
		case <-timeout:
			return // Success - no event received
		}
	}
}

func Test_Watcher_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent(gomock.Any()).Return(componentLog).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	cfg := &config.Config{}
	cfg.Logs.Buffer = 10

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	w, err := NewWatcher(config.DefaultConfig(), b, mockLog)
	require.NoError(t, err)

	w.Close()
	w.Close() // Should not panic on double close
}

func Test_Watcher_SkipsServiceWithoutWatchConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent(gomock.Any()).Return(componentLog).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"no-watch-service": {
				Dir:   tmpDir,
				Watch: nil,
			},
		},
	}
	cfg.Logs.Buffer = 10

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	w, err := NewWatcher(cfg, b, mockLog)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := b.Subscribe(ctx)
	w.Start(ctx)

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "no-watch-service", Tier: "default"}},
	})

	// Wait briefly - no WatchStarted event should be published
	timeout := time.After(100 * time.Millisecond)

	for {
		select {
		case event := <-eventCh:
			if event.Type == bus.EventWatchStarted {
				t.Fatal("should not start watching service without watch config")
			}
		case <-timeout:
			// Verify watcher was not registered
			m := w.(*manager)
			m.mu.RLock()
			_, exists := m.watchers["no-watch-service"]
			m.mu.RUnlock()
			assert.False(t, exists, "watcher should not be registered for service without watch config")

			return
		}
	}
}

func Test_Watcher_PublishesWatchStartedEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent(gomock.Any()).Return(componentLog).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: tmpDir,
				Watch: &config.Watch{
					Include: []string{"*.go"},
				},
			},
		},
	}
	cfg.Logs.Buffer = 10

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	w, err := NewWatcher(cfg, b, mockLog)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := b.Subscribe(ctx)
	w.Start(ctx)

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "test-service", Tier: "default"}},
	})

	timeout := time.After(time.Second)

	for {
		select {
		case event := <-eventCh:
			if event.Type == bus.EventWatchStarted {
				data, ok := event.Data.(bus.Payload)
				assert.True(t, ok)
				assert.Equal(t, "test-service", data.Name)

				return
			}
		case <-timeout:
			t.Fatal("timeout waiting for watch started event")
		}
	}
}

func Test_Watcher_PublishesWatchStoppedEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent(gomock.Any()).Return(componentLog).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: tmpDir,
				Watch: &config.Watch{
					Include: []string{"*.go"},
				},
			},
		},
	}
	cfg.Logs.Buffer = 10

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	w, err := NewWatcher(cfg, b, mockLog)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := b.Subscribe(ctx)
	w.Start(ctx)

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "test-service", Tier: "default"}},
	})

	waitForEvent(t, eventCh, bus.EventWatchStarted)

	b.Publish(bus.Message{
		Type: bus.EventServiceStopped,
		Data: bus.ServiceStopped{ServiceEvent: bus.ServiceEvent{Service: "test-service", Tier: "default"}},
	})

	timeout := time.After(time.Second)

	for {
		select {
		case event := <-eventCh:
			if event.Type == bus.EventWatchStopped {
				data, ok := event.Data.(bus.Payload)
				assert.True(t, ok)
				assert.Equal(t, "test-service", data.Name)

				return
			}
		case <-timeout:
			t.Fatal("timeout waiting for watch stopped event")
		}
	}
}

func Test_Watcher_IgnoreSkipsDirs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent(gomock.Any()).Return(componentLog).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	tmpDir := t.TempDir()

	skippedDir := filepath.Join(tmpDir, ".git")
	require.NoError(t, os.Mkdir(skippedDir, 0755))

	watchedDir := filepath.Join(tmpDir, "src")
	require.NoError(t, os.Mkdir(watchedDir, 0755))

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: tmpDir,
				Watch: &config.Watch{
					Include: []string{"**/*.go"},
					Ignore:  []string{".git/**"},
				},
			},
		},
	}
	cfg.Logs.Buffer = 10

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	w, err := NewWatcher(cfg, b, mockLog)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := b.Subscribe(ctx)
	w.Start(ctx)

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "test-service", Tier: "default"}},
	})

	waitForEvent(t, eventCh, bus.EventWatchStarted)

	m := w.(*manager)
	m.mu.RLock()
	watcher, exists := m.watchers["test-service"]
	m.mu.RUnlock()

	require.True(t, exists)

	for _, dir := range watcher.dirs {
		assert.NotContains(t, dir, ".git", "should not watch .git directory")
	}

	assert.Contains(t, watcher.dirs, watchedDir, "should watch src directory")
}

func Test_Watcher_WatchesSharedDirs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent(gomock.Any()).Return(componentLog).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	serviceDir := t.TempDir()
	sharedDir := t.TempDir()

	sharedFile := filepath.Join(sharedDir, "shared.go")
	err := os.WriteFile(sharedFile, []byte("package shared"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: serviceDir,
				Watch: &config.Watch{
					Include:  []string{"*.go", "**/*.go"},
					Shared:   []string{sharedDir},
					Debounce: 10 * time.Millisecond,
				},
			},
		},
	}
	cfg.Logs.Buffer = 10

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	w, err := NewWatcher(cfg, b, mockLog)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := b.Subscribe(ctx)
	w.Start(ctx)

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "test-service", Tier: "default"}},
	})

	waitForEvent(t, eventCh, bus.EventWatchStarted)

	err = os.WriteFile(sharedFile, []byte("package shared\n// modified"), 0644)
	require.NoError(t, err)

	timeout := time.After(3 * time.Second)

	for {
		select {
		case event := <-eventCh:
			if event.Type == bus.EventWatchTriggered {
				data, ok := event.Data.(bus.WatchTriggered)
				assert.True(t, ok)
				assert.Equal(t, "test-service", data.Service)
				assert.NotEmpty(t, data.ChangedFiles)

				return
			}
		case <-timeout:
			t.Fatal("timeout waiting for watch event from shared directory")
		}
	}
}

func Test_Watcher_IgnoreSkipsCustomDirs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	componentLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent(gomock.Any()).Return(componentLog).AnyTimes()
	componentLog.EXPECT().Info().Return(nil).AnyTimes()
	componentLog.EXPECT().Warn().Return(nil).AnyTimes()
	componentLog.EXPECT().Error().Return(nil).AnyTimes()

	mockServer := logs.NewMockServer(ctrl)
	mockServer.EXPECT().Broadcast(gomock.Any(), gomock.Any()).AnyTimes()

	tmpDir := t.TempDir()

	customDir := filepath.Join(tmpDir, "build")
	require.NoError(t, os.Mkdir(customDir, 0755))

	srcDir := filepath.Join(tmpDir, "src")
	require.NoError(t, os.Mkdir(srcDir, 0755))

	cfg := &config.Config{
		Services: map[string]*config.Service{
			"test-service": {
				Dir: tmpDir,
				Watch: &config.Watch{
					Include: []string{"**/*.go"},
					Ignore:  []string{"build/**"},
				},
			},
		},
	}
	cfg.Logs.Buffer = 10

	b := bus.New(cfg, mockServer, nil)
	defer b.Close()

	w, err := NewWatcher(cfg, b, mockLog)
	require.NoError(t, err)

	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := b.Subscribe(ctx)
	w.Start(ctx)

	b.Publish(bus.Message{
		Type: bus.EventServiceReady,
		Data: bus.ServiceReady{ServiceEvent: bus.ServiceEvent{Service: "test-service", Tier: "default"}},
	})

	waitForEvent(t, eventCh, bus.EventWatchStarted)

	m := w.(*manager)
	m.mu.RLock()
	watcher, exists := m.watchers["test-service"]
	m.mu.RUnlock()

	require.True(t, exists)

	for _, dir := range watcher.dirs {
		assert.NotContains(t, dir, "build", "should not watch build directory")
	}

	assert.Contains(t, watcher.dirs, srcDir, "should watch src directory")
}

func Test_normalizeSharedPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain directory path unchanged",
			input:    "examples/bookstore/common",
			expected: "examples/bookstore/common",
		},
		{
			name:     "strips trailing slash double star",
			input:    "examples/bookstore/common/**",
			expected: "examples/bookstore/common",
		},
		{
			name:     "strips trailing double star only",
			input:    "examples/bookstore/common**",
			expected: "examples/bookstore/common",
		},
		{
			name:     "strips trailing slash",
			input:    "examples/bookstore/common/",
			expected: "examples/bookstore/common",
		},
		{
			name:     "handles multiple trailing patterns",
			input:    "pkg/shared/**",
			expected: "pkg/shared",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeSharedPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// waitForEvent waits for a specific event type with timeout
func waitForEvent(t *testing.T, eventCh <-chan bus.Message, eventType bus.MessageType) {
	t.Helper()

	timer := time.After(time.Second)

	for {
		select {
		case event := <-eventCh:
			if event.Type == eventType {
				return
			}
		case <-timer:
			t.Fatalf("timeout waiting for event %s", eventType)
		}
	}
}
