package watch

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// queue is a Subscription over a plain channel
type queue chan contracts.Message

func (q queue) Messages() <-chan contracts.Message {
	return q
}

// isWatchTriggered matches the message a debounced file change publishes
func isWatchTriggered(msg contracts.Message) bool {
	return msg.Type == contracts.EventWatchTriggered
}

func Test_NewWatcher(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	w := NewWatcher(nil, nil, log)

	require.NotNil(t, w)
	assert.Nil(t, w.fsWatcher)
}

func Test_Watcher_Subscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	log := slog.New(slog.DiscardHandler)
	subscription := contracts.SubscribeOptions{Name: "watcher", Required: true, Types: serviceTypes}

	w := NewWatcher(nil, mockSubscriber, log)
	defer w.close()

	require.NoError(t, w.Start(t.Context()))

	tests := []struct {
		name     string
		before   func()
		expected error
	}{
		{
			name: "registers the required service subscription",
			before: func() {
				messages := make(queue)
				close(messages)

				mockSubscriber.EXPECT().Subscribe(gomock.Any(), subscription).Return(messages, nil)
			},
		},
		{
			name: "returns the subscribe error",
			before: func() {
				mockSubscriber.EXPECT().Subscribe(gomock.Any(), subscription).Return(nil, contracts.ErrBusClosed)
			},
			expected: contracts.ErrBusClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := w.Subscribe(t.Context())

			require.ErrorIs(t, err, tt.expected)
		})
	}
}

func Test_Watcher_Drain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)

	log := slog.New(slog.DiscardHandler)
	subscription := contracts.SubscribeOptions{Name: "watcher", Required: true, Types: serviceTypes}

	messages := make(queue)
	close(messages)

	w := NewWatcher(nil, mockSubscriber, log)

	mockSubscriber.EXPECT().Subscribe(gomock.Any(), subscription).Return(messages, nil)

	require.NoError(t, w.Subscribe(t.Context()))

	err := w.Drain(t.Context())

	require.NoError(t, err)
}

func Test_Watcher_publishTriggered(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockLog := NewMockLogger(ctrl)

	api := model.Service{ID: "test-id-api", Name: "api"}
	files := []string{"main.go"}
	change := contracts.Message{Type: contracts.EventWatchTriggered, Data: contracts.WatchTriggered{Service: api, ChangedFiles: files}}

	w := NewWatcher(mockPublisher, nil, mockLog)

	tests := []struct {
		name   string
		before func()
	}{
		{
			name: "publishes the batch",
			before: func() {
				mockPublisher.EXPECT().Publish(change).Return(nil)
			},
		},
		{
			name: "logs a failed publish",
			before: func() {
				mockPublisher.EXPECT().Publish(change).Return(contracts.ErrBusOverloaded)
				mockLog.EXPECT().Warn("Failed to publish the file change for service 'api'", "error", contracts.ErrBusOverloaded)
			},
		},
		{
			name: "a closed watcher publishes nothing",
			before: func() {
				w.closed = true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			w.publishTriggered(api, files)
		})
	}
}

func Test_Watcher_ServiceReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	tmpDir := t.TempDir()
	log := slog.New(slog.DiscardHandler)

	watched := model.Service{
		ID:        "test-id-svc",
		Name:      "test-service",
		Directory: tmpDir,
		Watch: &model.Watch{
			Include: []string{"*.go", "**/*.go"},
		},
	}
	plain := model.Service{ID: "test-id-plain", Name: "plain-service", Directory: tmpDir}

	w := NewWatcher(mockPublisher, nil, log)

	defer w.close()

	require.NoError(t, w.Start(t.Context()))

	tests := []struct {
		name     string
		before   func()
		msg      contracts.Message
		expected bool
	}{
		{
			name: "starts watching a service with a watch config",
			before: func() {
				mockPublisher.EXPECT().Publish(contracts.Message{
					Type: contracts.EventWatchStarted,
					Data: contracts.WatchStarted{Service: watched},
				}).Return(nil)
			},
			msg: contracts.Message{
				Type: contracts.EventServiceReady,
				Data: contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: watched, Tier: "default"}},
			},
			expected: true,
		},
		{
			name:   "skips a service without a watch config",
			before: func() {},
			msg: contracts.Message{
				Type: contracts.EventServiceReady,
				Data: contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: plain, Tier: "default"}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			w.handleServiceEvent(tt.msg)

			w.mu.RLock()
			_, exists := w.targets[tt.msg.Data.(contracts.ServiceReady).Service.ID]
			w.mu.RUnlock()

			assert.Equal(t, tt.expected, exists)
		})
	}
}

func Test_Watcher_ServiceStopped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	tmpDir := t.TempDir()
	log := slog.New(slog.DiscardHandler)

	api := model.Service{
		ID:        "test-id-svc",
		Name:      "test-service",
		Directory: tmpDir,
		Watch: &model.Watch{
			Include: []string{"*.go"},
		},
	}

	w := NewWatcher(mockPublisher, nil, log)

	defer w.close()

	require.NoError(t, w.Start(t.Context()))

	gomock.InOrder(
		mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStarted, Data: contracts.WatchStarted{Service: api}}).Return(nil),
		mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStopped, Data: contracts.WatchStopped{Service: api}}).Return(nil),
	)

	w.handleServiceEvent(contracts.Message{
		Type: contracts.EventServiceReady,
		Data: contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: api, Tier: "default"}},
	})
	w.handleServiceEvent(contracts.Message{
		Type: contracts.EventServiceStopped,
		Data: contracts.ServiceStopped{ServiceEvent: contracts.ServiceEvent{Service: api, Tier: "default"}},
	})

	w.mu.RLock()
	_, exists := w.targets["test-id-svc"]
	w.mu.RUnlock()

	assert.False(t, exists, "watcher should be removed after service stopped")
}

func Test_Watcher_stopWatching_Unknown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockLog := NewMockLogger(ctrl)

	api := &target{svc: model.Service{ID: "test-id-api", Name: "api"}}

	w := NewWatcher(mockPublisher, nil, mockLog)
	w.targets[api.svc.ID] = api

	w.stopWatching("test-id-unknown")

	assert.Equal(t, map[string]*target{"test-id-api": api}, w.targets)
}

func Test_Watcher_startWatching_ClosedDuringTheWalk(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockLog := NewMockLogger(ctrl)

	tmpDir := t.TempDir()
	missingDir := filepath.Join(t.TempDir(), "missing")

	api := model.Service{
		ID:        "test-id-svc",
		Name:      "test-service",
		Directory: tmpDir,
		Watch: &model.Watch{
			Include: []string{"**/*.go"},
			Shared:  []string{missingDir},
		},
	}

	w := NewWatcher(mockPublisher, nil, mockLog)
	require.NoError(t, w.Start(t.Context()))

	closeWatcher := func(string, ...any) {
		w.close()
	}

	mockLog.EXPECT().Warn(fmt.Sprintf("Failed to add shared directory '%s' for service 'test-service'", missingDir), "error", gomock.Any()).Do(closeWatcher)

	w.startWatching(api)

	assert.Empty(t, w.targets)
	assert.Empty(t, w.registry)
}

func Test_Watcher_StartWatching_RegistersAnotherServiceDuringTheWalk(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockLog := NewMockLogger(ctrl)

	firstDir := t.TempDir()
	secondDir := t.TempDir()
	missingDir := filepath.Join(t.TempDir(), "missing")

	first := model.Service{
		ID:        "test-id-svc1",
		Name:      "service-1",
		Directory: firstDir,
		Watch: &model.Watch{
			Include: []string{"**/*.go"},
			Shared:  []string{missingDir},
		},
	}
	second := model.Service{
		ID:        "test-id-svc2",
		Name:      "service-2",
		Directory: secondDir,
		Watch: &model.Watch{
			Include: []string{"**/*.go"},
		},
	}

	w := NewWatcher(mockPublisher, nil, mockLog)

	defer w.close()

	require.NoError(t, w.Start(t.Context()))

	registerSecond := func(string, ...any) {
		w.startWatching(second)
	}

	gomock.InOrder(
		mockLog.EXPECT().Warn(fmt.Sprintf("Failed to add shared directory '%s' for service 'service-1'", missingDir), "error", gomock.Any()).Do(registerSecond),
		mockLog.EXPECT().Info("Started watching service 'service-2' in "+secondDir),
		mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStarted, Data: contracts.WatchStarted{Service: second}}).Return(nil),
		mockLog.EXPECT().Info("Started watching service 'service-1' in "+firstDir),
		mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStarted, Data: contracts.WatchStarted{Service: first}}).Return(nil),
	)

	w.startWatching(first)

	assert.Len(t, w.targets, 2)
	assert.Equal(t, map[string][]string{firstDir: {first.ID}, secondDir: {second.ID}}, w.registry)
}

func Test_Watcher_StopWatching_DropsThePendingChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	tmpDir := t.TempDir()
	log := slog.New(slog.DiscardHandler)

	api := model.Service{
		ID:        "test-id-svc",
		Name:      "test-service",
		Directory: tmpDir,
		Watch: &model.Watch{
			Include:  []string{"*.go"},
			Debounce: 10 * time.Millisecond,
		},
	}

	w := NewWatcher(mockPublisher, nil, log)

	defer w.close()

	require.NoError(t, w.Start(t.Context()))

	gomock.InOrder(
		mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStarted, Data: contracts.WatchStarted{Service: api}}).Return(nil),
		mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStopped, Data: contracts.WatchStopped{Service: api}}).Return(nil),
	)

	synctest.Test(t, func(t *testing.T) {
		w.startWatching(api)
		w.targets[api.ID].debouncer.trigger("main.go")

		w.stopWatching(api.ID)

		<-time.After(time.Second)

		assert.Empty(t, w.targets)
	})
}

func Test_Watcher_PublishesEventOnFileChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	tmpDir := t.TempDir()
	log := slog.New(slog.DiscardHandler)

	testFile := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package main"), 0644))

	ignoredFile := filepath.Join(tmpDir, "main_test.go")
	require.NoError(t, os.WriteFile(ignoredFile, []byte("package main"), 0644))

	api := model.Service{
		ID:        "test-id-svc",
		Name:      "test-service",
		Directory: tmpDir,
		Watch: &model.Watch{
			Include:  []string{"*.go", "**/*.go"},
			Ignore:   []string{"*_test.go"},
			Debounce: 10 * time.Millisecond,
		},
	}
	triggered := make(chan contracts.WatchTriggered, 8)
	forwardTrigger := func(msg contracts.Message) error {
		triggered <- msg.Data.(contracts.WatchTriggered)

		return nil
	}

	w := NewWatcher(mockPublisher, nil, log)

	defer w.close()

	require.NoError(t, w.Start(t.Context()))

	mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStarted, Data: contracts.WatchStarted{Service: api}}).Return(nil)
	mockPublisher.EXPECT().Publish(gomock.Cond(isWatchTriggered)).DoAndReturn(forwardTrigger).AnyTimes()

	w.handleServiceEvent(contracts.Message{
		Type: contracts.EventServiceReady,
		Data: contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: api, Tier: "default"}},
	})

	require.NoError(t, os.WriteFile(ignoredFile, []byte("package main\n// modified"), 0644))
	require.NoError(t, os.WriteFile(testFile, []byte("package main\n// modified"), 0644))

	change := <-triggered

	assert.Equal(t, api, change.Service)
	assert.Equal(t, []string{"main.go"}, change.ChangedFiles, "the ignored test file is not part of the change")
}

func Test_Watcher_Close(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	w := NewWatcher(nil, nil, log)
	require.NoError(t, w.Start(t.Context()))

	w.close()
	w.close()
}

func Test_Watcher_IgnoreSkipsDirs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	tmpDir := t.TempDir()
	log := slog.New(slog.DiscardHandler)

	gitDir := filepath.Join(tmpDir, ".git")
	require.NoError(t, os.Mkdir(gitDir, 0755))

	buildDir := filepath.Join(tmpDir, "build")
	require.NoError(t, os.Mkdir(buildDir, 0755))

	srcDir := filepath.Join(tmpDir, "src")
	require.NoError(t, os.Mkdir(srcDir, 0755))

	api := model.Service{
		ID:        "test-id-svc",
		Name:      "test-service",
		Directory: tmpDir,
		Watch: &model.Watch{
			Include: []string{"**/*.go"},
			Ignore:  []string{".git/**", "build/**"},
		},
	}

	w := NewWatcher(mockPublisher, nil, log)

	defer w.close()

	require.NoError(t, w.Start(t.Context()))

	mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStarted, Data: contracts.WatchStarted{Service: api}}).Return(nil)

	w.handleServiceEvent(contracts.Message{
		Type: contracts.EventServiceReady,
		Data: contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: api, Tier: "default"}},
	})

	w.mu.RLock()
	target, exists := w.targets["test-id-svc"]
	w.mu.RUnlock()

	require.True(t, exists)
	assert.ElementsMatch(t, []string{tmpDir, srcDir}, target.list)
}

func Test_Watcher_WatchesSharedDirs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	serviceDir := t.TempDir()
	sharedDir := t.TempDir()
	log := slog.New(slog.DiscardHandler)

	sharedFile := filepath.Join(sharedDir, "shared.go")
	require.NoError(t, os.WriteFile(sharedFile, []byte("package shared"), 0644))

	api := model.Service{
		ID:        "test-id-svc",
		Name:      "test-service",
		Directory: serviceDir,
		Watch: &model.Watch{
			Include:  []string{"*.go", "**/*.go"},
			Shared:   []string{sharedDir},
			Debounce: 10 * time.Millisecond,
		},
	}
	triggered := make(chan contracts.WatchTriggered, 8)
	forwardTrigger := func(msg contracts.Message) error {
		triggered <- msg.Data.(contracts.WatchTriggered)

		return nil
	}

	w := NewWatcher(mockPublisher, nil, log)

	defer w.close()

	require.NoError(t, w.Start(t.Context()))

	mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStarted, Data: contracts.WatchStarted{Service: api}}).Return(nil)
	mockPublisher.EXPECT().Publish(gomock.Cond(isWatchTriggered)).DoAndReturn(forwardTrigger).AnyTimes()

	w.handleServiceEvent(contracts.Message{
		Type: contracts.EventServiceReady,
		Data: contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: api, Tier: "default"}},
	})

	require.NoError(t, os.WriteFile(sharedFile, []byte("package shared\n// modified"), 0644))

	change := <-triggered

	assert.Equal(t, api, change.Service)
	assert.Equal(t, []string{"shared.go"}, change.ChangedFiles)
}

func Test_Watcher_WatchesNewDirCreatedAtRuntime(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	tmpDir := t.TempDir()
	log := slog.New(slog.DiscardHandler)

	api := model.Service{
		ID:        "test-id-svc",
		Name:      "test-service",
		Directory: tmpDir,
		Watch: &model.Watch{
			Include:  []string{"**/*.go"},
			Debounce: 10 * time.Millisecond,
		},
	}
	newDir := filepath.Join(tmpDir, "newpkg")
	triggered := make(chan contracts.WatchTriggered, 8)
	forwardTrigger := func(msg contracts.Message) error {
		triggered <- msg.Data.(contracts.WatchTriggered)

		return nil
	}

	w := NewWatcher(mockPublisher, nil, log)

	defer w.close()

	require.NoError(t, w.Start(t.Context()))

	registered := func() bool {
		w.mu.RLock()
		defer w.mu.RUnlock()

		_, exists := w.registry[newDir]

		return exists
	}

	mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStarted, Data: contracts.WatchStarted{Service: api}}).Return(nil)
	mockPublisher.EXPECT().Publish(gomock.Cond(isWatchTriggered)).DoAndReturn(forwardTrigger).AnyTimes()

	w.handleServiceEvent(contracts.Message{
		Type: contracts.EventServiceReady,
		Data: contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: api, Tier: "default"}},
	})

	require.NoError(t, os.Mkdir(newDir, 0755))
	require.Eventually(t, registered, 3*time.Second, 10*time.Millisecond, "new directory should be registered")

	require.NoError(t, os.WriteFile(filepath.Join(newDir, "new.go"), []byte("package newpkg"), 0644))

	change := <-triggered

	assert.Equal(t, api, change.Service)
	assert.Equal(t, []string{filepath.Join("newpkg", "new.go")}, change.ChangedFiles)
}

func Test_Watcher_NewDirInSharedPathRegistersAllServices(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)

	serviceDir1 := t.TempDir()
	serviceDir2 := t.TempDir()
	sharedDir := t.TempDir()
	log := slog.New(slog.DiscardHandler)

	first := model.Service{
		ID:        "test-id-svc1",
		Name:      "service-1",
		Directory: serviceDir1,
		Watch: &model.Watch{
			Include:  []string{"**/*.go"},
			Shared:   []string{sharedDir},
			Debounce: 10 * time.Millisecond,
		},
	}
	second := model.Service{
		ID:        "test-id-svc2",
		Name:      "service-2",
		Directory: serviceDir2,
		Watch: &model.Watch{
			Include:  []string{"**/*.go"},
			Shared:   []string{sharedDir},
			Debounce: 10 * time.Millisecond,
		},
	}
	newDir := filepath.Join(sharedDir, "newpkg")

	w := NewWatcher(mockPublisher, nil, log)

	defer w.close()

	require.NoError(t, w.Start(t.Context()))

	registeredForBoth := func() bool {
		w.mu.RLock()
		defer w.mu.RUnlock()

		return len(w.registry[newDir]) == 2
	}

	mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStarted, Data: contracts.WatchStarted{Service: first}}).Return(nil)
	mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStarted, Data: contracts.WatchStarted{Service: second}}).Return(nil)

	w.handleServiceEvent(contracts.Message{
		Type: contracts.EventServiceReady,
		Data: contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: first, Tier: "default"}},
	})
	w.handleServiceEvent(contracts.Message{
		Type: contracts.EventServiceReady,
		Data: contracts.ServiceReady{ServiceEvent: contracts.ServiceEvent{Service: second, Tier: "default"}},
	})

	require.NoError(t, os.Mkdir(newDir, 0755))
	require.Eventually(t, registeredForBoth, 3*time.Second, 10*time.Millisecond, "new directory should be registered for both services")

	w.mu.RLock()
	subscribers := w.registry[newDir]
	w.mu.RUnlock()

	assert.ElementsMatch(t, []string{"test-id-svc1", "test-id-svc2"}, subscribers)
}

func Test_Watcher_startWatching(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockLog := NewMockLogger(ctrl)

	tmpDir := t.TempDir()
	missingDir := filepath.Join(t.TempDir(), "missing")

	watched := model.Service{
		ID:        "test-id-svc",
		Name:      "test-service",
		Directory: tmpDir,
		Watch: &model.Watch{
			Include: []string{"**/*.go"},
		},
	}
	invalid := model.Service{
		ID:        "test-id-invalid",
		Name:      "invalid-service",
		Directory: tmpDir,
		Watch: &model.Watch{
			Include: []string{"[invalid"},
		},
	}
	missing := model.Service{
		ID:        "test-id-missing",
		Name:      "missing-service",
		Directory: missingDir,
		Watch: &model.Watch{
			Include: []string{"**/*.go"},
		},
	}

	w := NewWatcher(mockPublisher, nil, mockLog)

	defer w.close()

	require.NoError(t, w.Start(t.Context()))

	tests := []struct {
		name     string
		before   func()
		svc      model.Service
		expected bool
	}{
		{
			name: "invalid include pattern logs and skips the service",
			before: func() {
				mockLog.EXPECT().Warn("Failed to create matcher for service 'invalid-service'", "error", gomock.Any())
			},
			svc:      invalid,
			expected: false,
		},
		{
			name: "missing directory logs and skips the service",
			before: func() {
				mockLog.EXPECT().Warn("Failed to add directories for service 'missing-service'", "error", gomock.Any())
			},
			svc:      missing,
			expected: false,
		},
		{
			name: "already watched service is not walked or announced again",
			before: func() {
				mockLog.EXPECT().Info("Started watching service 'test-service' in " + tmpDir)
				mockPublisher.EXPECT().Publish(contracts.Message{Type: contracts.EventWatchStarted, Data: contracts.WatchStarted{Service: watched}}).Return(nil)

				w.startWatching(watched)
			},
			svc:      watched,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			w.startWatching(tt.svc)

			assert.Equal(t, tt.expected, w.targets[tt.svc.ID] != nil)
		})
	}
}

func Test_Watcher_startWatching_Closed(t *testing.T) {
	tmpDir := t.TempDir()
	log := slog.New(slog.DiscardHandler)

	api := model.Service{
		ID:        "test-id-svc",
		Name:      "test-service",
		Directory: tmpDir,
		Watch: &model.Watch{
			Include: []string{"**/*.go"},
		},
	}

	w := NewWatcher(nil, nil, log)
	require.NoError(t, w.Start(t.Context()))
	w.close()

	w.startWatching(api)

	assert.Empty(t, w.targets)
	assert.Empty(t, w.registry)
}

func Test_Watcher_processEvents_LogsErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := NewMockLogger(ctrl)

	errs := make(chan error, 1)
	errs <- assert.AnError

	close(errs)

	w := NewWatcher(nil, nil, mockLog)
	w.fsWatcher = &fsnotify.Watcher{Events: make(chan fsnotify.Event), Errors: errs}
	w.done = make(chan struct{})

	mockLog.EXPECT().Error("Watcher error", "error", assert.AnError)

	w.processEvents()

	_, open := <-w.done

	assert.False(t, open)
}

func Test_Watcher_handleEvent(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	dir := t.TempDir()
	elsewhere := t.TempDir()
	event := fsnotify.Event{Name: filepath.Join(dir, "main.go"), Op: fsnotify.Write}

	matcher, err := newMatcher([]string{"*.go"}, nil)
	require.NoError(t, err)

	w := NewWatcher(nil, nil, log)

	tests := []struct {
		name     string
		before   func() *debouncer
		expected map[string]struct{}
	}{
		{
			name: "skips a service without a target and triggers the next",
			before: func() *debouncer {
				d := newDebouncer(time.Hour, nil)
				t.Cleanup(d.stop)

				w.targets = map[string]*target{"test-id-api": {root: dir, matcher: matcher, debouncer: d}}
				w.registry = map[string][]string{dir: {"test-id-gone", "test-id-api"}}

				return d
			},
			expected: map[string]struct{}{"main.go": {}},
		},
		{
			name: "skips a service the path lies outside of",
			before: func() *debouncer {
				d := newDebouncer(time.Hour, nil)

				w.targets = map[string]*target{"test-id-api": {root: elsewhere, matcher: matcher, debouncer: d}}
				w.registry = map[string][]string{dir: {"test-id-api"}}

				return d
			},
			expected: map[string]struct{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.before()

			w.handleEvent(event)

			assert.Equal(t, tt.expected, d.files)
		})
	}
}

func Test_Watcher_findTargets(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	parent := t.TempDir()
	elsewhere := t.TempDir()
	src := filepath.Join(parent, "src")
	build := filepath.Join(parent, "build")
	file := filepath.Join(parent, "main.go")

	require.NoError(t, os.Mkdir(src, 0o755))
	require.NoError(t, os.Mkdir(build, 0o755))
	require.NoError(t, os.WriteFile(file, []byte("package main"), 0o644))

	matcher, err := newMatcher([]string{"**/*.go"}, []string{"build/**"})
	require.NoError(t, err)

	w := NewWatcher(nil, nil, log)
	w.targets = map[string]*target{
		"test-id-api": {root: parent, matcher: matcher},
		"test-id-web": {root: elsewhere, matcher: matcher},
	}
	w.registry = map[string][]string{parent: {"test-id-gone", "test-id-web", "test-id-api"}}

	tests := []struct {
		name         string
		path         string
		expectedPath string
		expected     []string
	}{
		{
			name:         "a new directory goes to every service it lies under",
			path:         src,
			expectedPath: src,
			expected:     []string{"test-id-api"},
		},
		{
			name:         "an ignored directory goes to no service",
			path:         build,
			expectedPath: build,
			expected:     []string{},
		},
		{
			name:         "a file is not a new directory",
			path:         file,
			expectedPath: "",
			expected:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, targets := w.findTargets(tt.path)

			assert.Equal(t, tt.expectedPath, path)
			assert.Equal(t, tt.expected, targets)
		})
	}
}

func Test_Watcher_registerRecursive_Closed(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	root := t.TempDir()

	matcher, err := newMatcher([]string{"**/*.go"}, nil)
	require.NoError(t, err)

	w := NewWatcher(nil, nil, log)
	require.NoError(t, w.Start(t.Context()))
	w.close()

	dirs, err := w.registerRecursive(root, matcher)

	require.ErrorIs(t, err, fsnotify.ErrClosed)
	assert.Empty(t, dirs)
}

func Test_Watcher_register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := NewMockLogger(ctrl)

	newDir := t.TempDir()
	missingDir := filepath.Join(t.TempDir(), "missing")

	api := model.Service{ID: "test-id-svc", Name: "test-service"}
	serviceIDs := []string{api.ID, "test-id-unknown"}

	w := NewWatcher(nil, nil, mockLog)

	defer w.close()

	require.NoError(t, w.Start(t.Context()))

	w.targets[api.ID] = &target{svc: api, debouncer: newDebouncer(time.Hour, nil)}

	tests := []struct {
		name             string
		before           func()
		path             string
		expectedRegistry []string
		expectedList     []string
	}{
		{
			name:             "adds the directory for every known service",
			before:           func() {},
			path:             newDir,
			expectedRegistry: []string{api.ID},
			expectedList:     []string{newDir},
		},
		{
			name: "missing directory logs and registers nothing",
			before: func() {
				w.targets[api.ID].list = nil

				mockLog.EXPECT().Warn("Failed to watch new directory: "+missingDir, "error", gomock.Any())
			},
			path:             missingDir,
			expectedRegistry: nil,
			expectedList:     nil,
		},
		{
			name: "a recreated directory is registered once",
			before: func() {
				w.registry[newDir] = []string{api.ID}
				w.targets[api.ID].list = []string{newDir}
			},
			path:             newDir,
			expectedRegistry: []string{api.ID},
			expectedList:     []string{newDir},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			w.register(tt.path, serviceIDs)

			assert.Equal(t, tt.expectedRegistry, w.registry[tt.path])
			assert.Equal(t, tt.expectedList, w.targets[api.ID].list)
		})
	}
}

func Test_Watcher_unregister(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	w := NewWatcher(nil, nil, log)

	tests := []struct {
		name             string
		before           func()
		expected         bool
		expectedRegistry map[string][]string
	}{
		{
			name: "untracked directory leaves fsnotify",
			before: func() {
				w.registry = map[string][]string{}
			},
			expected:         true,
			expectedRegistry: map[string][]string{},
		},
		{
			name: "last subscriber drops the directory",
			before: func() {
				w.registry = map[string][]string{"/srv/api": {"test-id-svc"}}
			},
			expected:         true,
			expectedRegistry: map[string][]string{},
		},
		{
			name: "remaining subscriber keeps the directory watched",
			before: func() {
				w.registry = map[string][]string{"/srv/api": {"test-id-other", "test-id-svc"}}
			},
			expected:         false,
			expectedRegistry: map[string][]string{"/srv/api": {"test-id-other"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			removed := w.unregister("/srv/api", "test-id-svc")

			assert.Equal(t, tt.expected, removed)
			assert.Equal(t, tt.expectedRegistry, w.registry)
		})
	}
}

func Test_relativeToBase(t *testing.T) {
	subject := &target{
		root:   "/srv/api",
		shared: []string{"relative/common", "/srv/common"},
	}

	tests := []struct {
		name       string
		path       string
		expected   string
		expectedOk bool
	}{
		{
			name:       "path under the root",
			path:       "/srv/api/cmd/main.go",
			expected:   filepath.Join("cmd", "main.go"),
			expectedOk: true,
		},
		{
			name:       "path under a shared base after one that cannot relate",
			path:       "/srv/common/lib/util.go",
			expected:   filepath.Join("lib", "util.go"),
			expectedOk: true,
		},
		{
			name:       "path outside every base",
			path:       "/srv/web/main.go",
			expected:   "",
			expectedOk: false,
		},
		{
			name:       "path under a root directory whose name starts with two dots",
			path:       "/srv/api/..foo/main.go",
			expected:   filepath.Join("..foo", "main.go"),
			expectedOk: true,
		},
		{
			name:       "path under a shared directory whose name starts with two dots",
			path:       "/srv/common/..foo/util.go",
			expected:   filepath.Join("..foo", "util.go"),
			expectedOk: true,
		},
		{
			name:       "parent of every base",
			path:       "/srv",
			expected:   "",
			expectedOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relPath, ok := relativeToBase(subject, tt.path)

			assert.Equal(t, tt.expected, relPath)
			assert.Equal(t, tt.expectedOk, ok)
		})
	}
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

func Test_Watcher_Stop(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	w := NewWatcher(nil, nil, log)
	require.NoError(t, w.Start(t.Context()))

	err := w.Stop(t.Context())

	require.NoError(t, err)
	assert.True(t, w.closed)

	select {
	case <-w.done:
	default:
		t.Fatal("Stop returned before the event goroutine exited")
	}
}
