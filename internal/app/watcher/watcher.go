package watcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"

	"fuku/internal/app/bus"
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
	name       string
	dir        string
	sharedDirs []string
	dirs       []string
	matcher    Matcher
	debouncer  Debouncer
	cancel     context.CancelFunc
}

// manager implements the Watcher interface
type manager struct {
	cfg       *config.Config
	bus       bus.Bus
	fsWatcher *fsnotify.Watcher
	watchers  map[string]*watcher
	mu        sync.RWMutex
	closed    bool
	log       logger.Logger
}

// NewWatcher creates a new Watcher instance
func NewWatcher(cfg *config.Config, b bus.Bus, log logger.Logger) (Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	m := &manager{
		cfg:       cfg,
		bus:       b,
		fsWatcher: fsw,
		watchers:  make(map[string]*watcher),
		log:       log.WithComponent("WATCHER"),
	}

	go m.processEvents()

	return m, nil
}

// Start subscribes to service events and manages file watching
func (m *manager) Start(ctx context.Context) {
	msgCh := m.bus.Subscribe(ctx)

	go func() {
		for msg := range msgCh {
			m.handleServiceEvent(ctx, msg)
		}
	}()
}

// handleServiceEvent processes bus messages to start/stop watching
func (m *manager) handleServiceEvent(ctx context.Context, msg bus.Message) {
	switch msg.Type {
	case bus.EventServiceReady:
		if data, ok := msg.Data.(bus.ServiceReady); ok {
			m.startWatching(ctx, data.Service)
		}
	case bus.EventServiceStopped:
		if data, ok := msg.Data.(bus.ServiceStopped); ok {
			m.stopWatching(data.Service)
		}
		// Note: We intentionally don't stop watching on EventServiceFailed.
		// This allows the next file change to trigger a restart attempt.
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

	matcher, err := NewMatcher(serviceCfg.Watch.Include, serviceCfg.Watch.Ignore)
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

	debounce := serviceCfg.Watch.Debounce
	if debounce == 0 {
		debounce = config.WatchDebounce
	}

	w.debouncer = NewDebouncer(debounce, func(files []string) {
		m.emitEvent(serviceName, files)
	})

	dirs, err := m.addDirRecursive(absDir, matcher)
	if err != nil {
		cancel()
		m.log.Warn().Err(err).Msgf("Failed to add directories for service '%s'", serviceName)

		return
	}

	w.dirs = dirs

	for _, shared := range serviceCfg.Watch.Shared {
		shared = normalizeSharedPath(shared)

		absShared, err := filepath.Abs(shared)
		if err != nil {
			m.log.Warn().Err(err).Msgf("Failed to resolve shared path '%s' for service '%s'", shared, serviceName)
			continue
		}

		w.sharedDirs = append(w.sharedDirs, absShared)

		sharedDirs, err := m.addDirRecursive(absShared, matcher)
		if err != nil {
			m.log.Warn().Err(err).Msgf("Failed to add shared directory '%s' for service '%s'", absShared, serviceName)
			continue
		}

		w.dirs = append(w.dirs, sharedDirs...)
	}

	m.watchers[serviceName] = w
	m.log.Info().Msgf("Started watching service '%s' in %s", serviceName, absDir)

	m.bus.Publish(bus.Message{
		Type: bus.EventWatchStarted,
		Data: bus.Payload{Name: serviceName},
	})

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
	m.removeDirs(w)
	delete(m.watchers, serviceName)
	m.log.Info().Msgf("Stopped watching service '%s'", serviceName)

	m.bus.Publish(bus.Message{
		Type: bus.EventWatchStopped,
		Data: bus.Payload{Name: serviceName},
	})
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

	var (
		newDirPath        string
		targetWatcherName string
	)

	m.mu.RLock()

	for _, w := range m.watchers {
		relPath, ok := relativeToAnyBaseDir(w, event.Name)
		if !ok {
			continue
		}

		if w.matcher.Match(relPath) {
			w.debouncer.Trigger(relPath)
		}
	}

	if event.Has(fsnotify.Create) {
		newDirPath, targetWatcherName = m.findNewDirTarget(event.Name)
	}

	m.mu.RUnlock()

	if targetWatcherName != "" {
		m.addNewDir(newDirPath, targetWatcherName)
	}
}

// findNewDirTarget checks if path is a new directory to watch (called under RLock)
func (m *manager) findNewDirTarget(path string) (string, string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", ""
	}

	for _, w := range m.watchers {
		relPath, ok := relativeToAnyBaseDir(w, path)
		if !ok {
			continue
		}

		if w.matcher.MatchDir(relPath) {
			continue
		}

		return path, w.name
	}

	return "", ""
}

// addNewDir adds a directory to the watch list (acquires write lock)
func (m *manager) addNewDir(path, watcherName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, exists := m.watchers[watcherName]
	if !exists {
		return
	}

	if err := m.fsWatcher.Add(path); err != nil {
		m.log.Warn().Err(err).Msgf("Failed to watch new directory: %s", path)
		return
	}

	w.dirs = append(w.dirs, path)
}

// relativeToAnyBaseDir returns a relative path from the first matching base dir (service dir or shared dirs)
func relativeToAnyBaseDir(w *watcher, path string) (string, bool) {
	if relPath, err := filepath.Rel(w.dir, path); err == nil && !strings.HasPrefix(relPath, "..") {
		return relPath, true
	}

	for _, baseDir := range w.sharedDirs {
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			continue
		}

		if !strings.HasPrefix(relPath, "..") {
			return relPath, true
		}
	}

	return "", false
}

// emitEvent publishes a watch triggered event to the bus
func (m *manager) emitEvent(serviceName string, files []string) {
	m.mu.RLock()
	closed := m.closed
	m.mu.RUnlock()

	if closed {
		return
	}

	m.bus.Publish(bus.Message{
		Type:     bus.EventWatchTriggered,
		Data:     bus.WatchTriggered{Service: serviceName, ChangedFiles: files},
		Critical: true,
	})
}

// addDirRecursive adds a directory and all subdirectories to the watch list and returns the added paths
func (m *manager) addDirRecursive(dir string, matcher Matcher) ([]string, error) {
	var dirs []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return nil
		}

		if path != dir {
			relPath, relErr := filepath.Rel(dir, path)
			if relErr != nil {
				return nil
			}

			if matcher.MatchDir(relPath) {
				return filepath.SkipDir
			}
		}

		if err := m.fsWatcher.Add(path); err != nil {
			m.log.Warn().Err(err).Msgf("Failed to watch directory: %s", path)
		} else {
			dirs = append(dirs, path)
		}

		return nil
	})

	return dirs, err
}

// isRelevantEvent returns true if the event should trigger a reload
func isRelevantEvent(event fsnotify.Event) bool {
	return event.Has(fsnotify.Write) ||
		event.Has(fsnotify.Create) ||
		event.Has(fsnotify.Remove) ||
		event.Has(fsnotify.Rename)
}

// removeDirs removes all tracked directories for a watcher from fsnotify
func (m *manager) removeDirs(w *watcher) {
	for _, dir := range w.dirs {
		_ = m.fsWatcher.Remove(dir)
	}
}

// normalizeSharedPath strips trailing glob suffixes from shared directory paths
func normalizeSharedPath(path string) string {
	for {
		switch {
		case strings.HasSuffix(path, "/**"):
			path = strings.TrimSuffix(path, "/**")
		case strings.HasSuffix(path, "**"):
			path = strings.TrimSuffix(path, "**")
		case strings.HasSuffix(path, "/"):
			path = strings.TrimSuffix(path, "/")
		default:
			return path
		}
	}
}
