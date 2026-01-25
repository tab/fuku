package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"

	"fuku/internal/app/runtime"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Watcher monitors file changes for services
type Watcher interface {
	Start(ctx context.Context)
	Close()
}

// watcher holds state for a single watched service
type watcher struct {
	name      string
	dir       string
	matcher   Matcher
	debouncer Debouncer
	cancel    context.CancelFunc
}

// manager implements the Watcher interface
type manager struct {
	cfg       *config.Config
	eventBus  runtime.EventBus
	fsWatcher *fsnotify.Watcher
	watchers  map[string]*watcher
	log       logger.Logger
	mu        sync.RWMutex
	closed    bool
}

// NewWatcher creates a new Watcher instance
func NewWatcher(cfg *config.Config, eventBus runtime.EventBus, log logger.Logger) (Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	m := &manager{
		cfg:       cfg,
		eventBus:  eventBus,
		fsWatcher: fsw,
		watchers:  make(map[string]*watcher),
		log:       log.WithComponent("WATCHER"),
	}

	go m.processEvents()

	return m, nil
}

// Start subscribes to service events and manages file watching
func (m *manager) Start(ctx context.Context) {
	eventCh := m.eventBus.Subscribe(ctx)

	go func() {
		for event := range eventCh {
			m.handleServiceEvent(ctx, event)
		}
	}()
}

// handleServiceEvent processes runtime events to start/stop watching
func (m *manager) handleServiceEvent(ctx context.Context, event runtime.Event) {
	switch event.Type {
	case runtime.EventServiceReady:
		if data, ok := event.Data.(runtime.ServiceReadyData); ok {
			m.startWatching(ctx, data.Service)
		}
	case runtime.EventServiceStopped:
		if data, ok := event.Data.(runtime.ServiceStoppedData); ok {
			m.stopWatching(data.Service)
		}
	case runtime.EventServiceFailed:
		if data, ok := event.Data.(runtime.ServiceFailedData); ok {
			m.stopWatching(data.Service)
		}
	}
}

// startWatching starts watching files for a service
func (m *manager) startWatching(ctx context.Context, serviceName string) {
	serviceCfg, exists := m.cfg.Services[serviceName]
	if !exists || serviceCfg.Watch == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}

	if _, exists := m.watchers[serviceName]; exists {
		return
	}

	matcher, err := NewMatcher(serviceCfg.Watch.Paths, serviceCfg.Watch.Ignore)
	if err != nil {
		m.log.Warn().Err(err).Msgf("Failed to create matcher for service '%s'", serviceName)
		return
	}

	absDir, err := filepath.Abs(serviceCfg.Dir)
	if err != nil {
		m.log.Warn().Err(err).Msgf("Failed to get absolute path for service '%s'", serviceName)
		return
	}

	ctx, cancel := context.WithCancel(ctx)

	w := &watcher{
		name:    serviceName,
		dir:     absDir,
		matcher: matcher,
		cancel:  cancel,
	}

	w.debouncer = NewDebouncer(config.WatchDebounce, func(files []string) {
		m.emitEvent(serviceName, files)
	})

	if err := m.addDirRecursive(absDir); err != nil {
		cancel()
		m.log.Warn().Err(err).Msgf("Failed to add directories for service '%s'", serviceName)

		return
	}

	m.watchers[serviceName] = w
	m.log.Info().Msgf("Started watching service '%s' in %s", serviceName, absDir)

	go func() {
		<-ctx.Done()
		w.debouncer.Stop()
	}()
}

// stopWatching stops watching files for a service
func (m *manager) stopWatching(serviceName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, exists := m.watchers[serviceName]
	if !exists {
		return
	}

	w.cancel()
	delete(m.watchers, serviceName)
	m.log.Info().Msgf("Stopped watching service '%s'", serviceName)
}

// Close stops the watcher and releases resources
func (m *manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}

	m.closed = true

	for name, w := range m.watchers {
		w.cancel()
		delete(m.watchers, name)
	}

	m.fsWatcher.Close()
}

// processEvents handles fsnotify events and routes them to watchers
func (m *manager) processEvents() {
	for {
		select {
		case event, ok := <-m.fsWatcher.Events:
			if !ok {
				return
			}

			m.handleEvent(event)
		case err, ok := <-m.fsWatcher.Errors:
			if !ok {
				return
			}

			m.log.Error().Err(err).Msg("Watcher error")
		}
	}
}

// handleEvent processes a single fsnotify event
func (m *manager) handleEvent(event fsnotify.Event) {
	if !isRelevantEvent(event) {
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, w := range m.watchers {
		relPath, err := filepath.Rel(w.dir, event.Name)
		if err != nil {
			continue
		}

		if len(relPath) > 2 && relPath[:2] == ".." {
			continue
		}

		if w.matcher.Match(relPath) {
			w.debouncer.Trigger(relPath)
		}
	}

	if event.Has(fsnotify.Create) {
		m.handleCreate(event.Name)
	}
}

// handleCreate adds newly created directories to the watch list
func (m *manager) handleCreate(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	if info.IsDir() {
		if err := m.fsWatcher.Add(path); err != nil {
			m.log.Warn().Err(err).Msgf("Failed to watch new directory: %s", path)
		}
	}
}

// emitEvent publishes a watch triggered event to the event bus
func (m *manager) emitEvent(serviceName string, files []string) {
	m.mu.RLock()
	closed := m.closed
	m.mu.RUnlock()

	if closed {
		return
	}

	m.eventBus.Publish(runtime.Event{
		Type:     runtime.EventWatchTriggered,
		Data:     runtime.WatchTriggeredData{Service: serviceName, ChangedFiles: files},
		Critical: true,
	})
}

// addDirRecursive adds a directory and all subdirectories to the watch list
func (m *manager) addDirRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}

			if err := m.fsWatcher.Add(path); err != nil {
				m.log.Warn().Err(err).Msgf("Failed to watch directory: %s", path)
			}
		}

		return nil
	})
}

// isRelevantEvent returns true if the event should trigger a reload
func isRelevantEvent(event fsnotify.Event) bool {
	return event.Has(fsnotify.Write) ||
		event.Has(fsnotify.Create) ||
		event.Has(fsnotify.Remove) ||
		event.Has(fsnotify.Rename)
}

// shouldSkipDir returns true if the directory should not be watched
func shouldSkipDir(name string) bool {
	skip := []string{".git", "node_modules", "vendor", ".idea", ".vscode"}

	for _, s := range skip {
		if name == s {
			return true
		}
	}

	return false
}
