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
	name      string
	root      string
	shared    []string
	list      []string
	matcher   Matcher
	debouncer Debouncer
	cancel    context.CancelFunc
}

// manager implements the Watcher interface
type manager struct {
	cfg       *config.Config
	bus       bus.Bus
	fsWatcher *fsnotify.Watcher
	watchers  map[string]*watcher
	registry  map[string][]string
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
		registry:  make(map[string][]string),
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

	root, err := filepath.Abs(serviceCfg.Dir)
	if err != nil {
		m.log.Warn().Err(err).Msgf("Failed to get absolute path for service '%s'", serviceName)
		return
	}

	ctx, cancel := context.WithCancel(ctx)

	w := &watcher{
		name:    serviceName,
		root:    root,
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

	dirs, err := m.registerRecursive(root, serviceName, matcher)
	if err != nil {
		cancel()
		m.log.Warn().Err(err).Msgf("Failed to add directories for service '%s'", serviceName)

		return
	}

	w.list = dirs

	for _, sharedPath := range serviceCfg.Watch.Shared {
		sharedPath = normalizeSharedPath(sharedPath)

		absShared, err := filepath.Abs(sharedPath)
		if err != nil {
			m.log.Warn().Err(err).Msgf("Failed to resolve shared path '%s' for service '%s'", sharedPath, serviceName)
			continue
		}

		w.shared = append(w.shared, absShared)

		sharedDirs, err := m.registerRecursive(absShared, serviceName, matcher)
		if err != nil {
			m.log.Warn().Err(err).Msgf("Failed to add shared directory '%s' for service '%s'", absShared, serviceName)
			continue
		}

		w.list = append(w.list, sharedDirs...)
	}

	m.watchers[serviceName] = w
	m.log.Info().Msgf("Started watching service '%s' in %s", serviceName, root)

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
	m.unregisterAll(w)
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
		newDirPath string
		targets    []string
	)

	dir := filepath.Dir(event.Name)

	m.mu.RLock()

	for _, serviceName := range m.registry[dir] {
		w, exists := m.watchers[serviceName]
		if !exists {
			continue
		}

		relPath, ok := relativeToBase(w, event.Name)
		if !ok {
			continue
		}

		if w.matcher.Match(relPath) {
			w.debouncer.Trigger(relPath)
		}
	}

	if event.Has(fsnotify.Create) {
		newDirPath, targets = m.findTargets(event.Name)
	}

	m.mu.RUnlock()

	if len(targets) > 0 {
		m.register(newDirPath, targets)
	}
}

// findTargets returns all services that should watch the new directory (called under RLock)
func (m *manager) findTargets(path string) (string, []string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", nil
	}

	parentDir := filepath.Dir(path)
	subscribers := m.registry[parentDir]
	targets := make([]string, 0, len(subscribers))

	for _, serviceName := range subscribers {
		w, exists := m.watchers[serviceName]
		if !exists {
			continue
		}

		relPath, ok := relativeToBase(w, path)
		if !ok {
			continue
		}

		if w.matcher.MatchDir(relPath) {
			continue
		}

		targets = append(targets, serviceName)
	}

	return path, targets
}

// register adds a directory to the watch list for specified services (acquires write lock)
func (m *manager) register(path string, serviceNames []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.fsWatcher.Add(path); err != nil {
		m.log.Warn().Err(err).Msgf("Failed to watch new directory: %s", path)
		return
	}

	for _, serviceName := range serviceNames {
		w, exists := m.watchers[serviceName]
		if !exists {
			continue
		}

		w.list = append(w.list, path)
		m.registry[path] = append(m.registry[path], serviceName)
	}
}

// relativeToBase returns a relative path from the first matching base (root or shared)
func relativeToBase(w *watcher, path string) (string, bool) {
	if relPath, err := filepath.Rel(w.root, path); err == nil && !strings.HasPrefix(relPath, "..") {
		return relPath, true
	}

	for _, base := range w.shared {
		relPath, err := filepath.Rel(base, path)
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

// registerRecursive adds a directory and all subdirectories to the watch list and returns the added paths
func (m *manager) registerRecursive(dir string, serviceName string, matcher Matcher) ([]string, error) {
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
			m.registry[path] = append(m.registry[path], serviceName)
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

// unregisterAll removes all tracked directories for a watcher from fsnotify and registry
func (m *manager) unregisterAll(w *watcher) {
	for _, dir := range w.list {
		if m.unregister(dir, w.name) {
			_ = m.fsWatcher.Remove(dir)
		}
	}
}

// unregister removes a service from a directory's registry, returning true if it should be removed from fsnotify
func (m *manager) unregister(dir, serviceName string) bool {
	subscribers := m.registry[dir]
	if len(subscribers) == 0 {
		return true
	}

	n := 0

	for _, name := range subscribers {
		if name != serviceName {
			subscribers[n] = name
			n++
		}
	}

	if n == 0 {
		delete(m.registry, dir)
		return true
	}

	m.registry[dir] = subscribers[:n]

	return false
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
