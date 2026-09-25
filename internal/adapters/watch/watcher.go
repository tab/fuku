package watch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// DefaultDebounce is the restart delay after a change for a service that sets none
const DefaultDebounce = 500 * time.Millisecond

// target holds state for a single watched service
type target struct {
	svc       model.Service
	root      string
	shared    []string
	list      []string
	matcher   *matcher
	debouncer *debouncer
}

// Logger is the logging surface the watcher writes through
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Watcher monitors file changes for services and publishes them as watch events
type Watcher struct {
	publisher  contracts.Publisher
	subscriber contracts.Subscriber
	fsWatcher  *fsnotify.Watcher
	loop       *contracts.Loop
	targets    map[string]*target
	registry   map[string][]string
	mu         sync.RWMutex
	closed     bool
	done       chan struct{}
	log        Logger
}

// NewWatcher creates a watcher that holds no file system resource until Start
func NewWatcher(publisher contracts.Publisher, subscriber contracts.Subscriber, log Logger) *Watcher {
	return &Watcher{
		publisher:  publisher,
		subscriber: subscriber,
		targets:    make(map[string]*target),
		registry:   make(map[string][]string),
		log:        log,
	}
}

// Start opens the fsnotify watcher and routes its events to the watched services on its own goroutine
func (w *Watcher) Start(context.Context) error {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create the file watcher: %w", err)
	}

	w.mu.Lock()
	w.fsWatcher = fsWatcher
	w.done = make(chan struct{})
	w.mu.Unlock()

	go w.processEvents()

	return nil
}

// Stop closes the watcher and releases its resources
func (w *Watcher) Stop(context.Context) error {
	w.close()

	return nil
}

// startWatching walks the directories of a service outside the lock, then stores its target and registry entries
func (w *Watcher) startWatching(svc model.Service) {
	if svc.Watch == nil {
		return
	}

	w.mu.RLock()
	_, exists := w.targets[svc.ID]
	closed := w.closed
	w.mu.RUnlock()

	if closed || exists {
		return
	}

	matcher, err := newMatcher(svc.Watch.Include, svc.Watch.Ignore)
	if err != nil {
		w.log.Warn(fmt.Sprintf("Failed to create matcher for service '%s'", svc.Name), "error", err)
		return
	}

	root, err := filepath.Abs(svc.Directory)
	if err != nil {
		w.log.Warn(fmt.Sprintf("Failed to get absolute path for service '%s'", svc.Name), "error", err)
		return
	}

	t := &target{
		svc:     svc,
		root:    root,
		matcher: matcher,
	}

	debounce := svc.Watch.Debounce
	if debounce == 0 {
		debounce = DefaultDebounce
	}

	t.debouncer = newDebouncer(debounce, func(files []string) {
		w.publishTriggered(svc, files)
	})

	dirs, err := w.registerRecursive(root, matcher)
	if err != nil {
		w.log.Warn(fmt.Sprintf("Failed to add directories for service '%s'", svc.Name), "error", err)

		return
	}

	t.list = dirs

	for _, sharedPath := range svc.Watch.Shared {
		sharedPath = normalizeSharedPath(sharedPath)

		absShared, err := filepath.Abs(sharedPath)
		if err != nil {
			w.log.Warn(fmt.Sprintf("Failed to resolve shared path '%s' for service '%s'", sharedPath, svc.Name), "error", err)
			continue
		}

		t.shared = append(t.shared, absShared)

		sharedDirs, err := w.registerRecursive(absShared, matcher)
		if err != nil {
			w.log.Warn(fmt.Sprintf("Failed to add shared directory '%s' for service '%s'", absShared, svc.Name), "error", err)
			continue
		}

		t.list = append(t.list, sharedDirs...)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return
	}

	for _, dir := range t.list {
		w.registry[dir] = append(w.registry[dir], svc.ID)
	}

	w.targets[svc.ID] = t
	w.log.Info(fmt.Sprintf("Started watching service '%s' in %s", svc.Name, root))
	w.publishWatching(contracts.EventWatchStarted, contracts.WatchStarted{Service: svc})
}

// stopWatching stops watching files for a service and cancels its pending batch before it returns
func (w *Watcher) stopWatching(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	t, exists := w.targets[id]
	if !exists {
		return
	}

	t.debouncer.stop()
	w.unregisterAll(t)
	delete(w.targets, id)
	w.log.Info(fmt.Sprintf("Stopped watching service '%s'", t.svc.Name))
	w.publishWatching(contracts.EventWatchStopped, contracts.WatchStopped{Service: t.svc})
}

// close stops the watcher, releases resources and waits for the event goroutine to exit
func (w *Watcher) close() {
	if !w.release() {
		return
	}

	// sync: wait outside w.mu, the event goroutine takes it to route an event in flight
	<-w.done
}

// release marks the watcher closed, stops every target and closes fsnotify, reporting whether it did so first
func (w *Watcher) release() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return false
	}

	w.closed = true

	for id, t := range w.targets {
		t.debouncer.stop()
		delete(w.targets, id)
	}

	w.fsWatcher.Close()

	return true
}

// processEvents handles fsnotify events and routes them to the watched services until fsnotify closes
func (w *Watcher) processEvents() {
	defer close(w.done)

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			w.handleEvent(event)
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}

			w.log.Error("Watcher error", "error", err)
		}
	}
}

// handleEvent processes a single fsnotify event
func (w *Watcher) handleEvent(event fsnotify.Event) {
	if !isRelevantEvent(event) {
		return
	}

	var (
		newDirPath string
		targets    []string
	)

	dir := filepath.Dir(event.Name)

	w.mu.RLock()

	for _, serviceID := range w.registry[dir] {
		t, exists := w.targets[serviceID]
		if !exists {
			continue
		}

		relPath, ok := relativeToBase(t, event.Name)
		if !ok {
			continue
		}

		if t.matcher.match(relPath) {
			t.debouncer.trigger(relPath)
		}
	}

	if event.Has(fsnotify.Create) {
		newDirPath, targets = w.findTargets(event.Name)
	}

	w.mu.RUnlock()

	if len(targets) > 0 {
		w.register(newDirPath, targets)
	}
}

// findTargets returns all service IDs that should watch the new directory (called under RLock)
func (w *Watcher) findTargets(path string) (string, []string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", nil
	}

	parentDir := filepath.Dir(path)
	subscribers := w.registry[parentDir]
	targets := make([]string, 0, len(subscribers))

	for _, serviceID := range subscribers {
		t, exists := w.targets[serviceID]
		if !exists {
			continue
		}

		relPath, ok := relativeToBase(t, path)
		if !ok {
			continue
		}

		if t.matcher.matchDir(relPath) {
			continue
		}

		targets = append(targets, serviceID)
	}

	return path, targets
}

// register adds a directory to the watch list for specified services (acquires write lock)
func (w *Watcher) register(path string, serviceIDs []string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.fsWatcher.Add(path); err != nil {
		w.log.Warn("Failed to watch new directory: "+path, "error", err)
		return
	}

	for _, serviceID := range serviceIDs {
		t, exists := w.targets[serviceID]
		if !exists || slices.Contains(w.registry[path], serviceID) {
			continue
		}

		t.list = append(t.list, path)
		w.registry[path] = append(w.registry[path], serviceID)
	}
}

// relativeToBase returns a relative path from the first matching base (root or shared)
func relativeToBase(t *target, path string) (string, bool) {
	if relPath, err := filepath.Rel(t.root, path); err == nil && relPath != ".." && !strings.HasPrefix(relPath, "../") {
		return relPath, true
	}

	for _, base := range t.shared {
		relPath, err := filepath.Rel(base, path)
		if err != nil {
			continue
		}

		if relPath != ".." && !strings.HasPrefix(relPath, "../") {
			return relPath, true
		}
	}

	return "", false
}

// registerRecursive adds a directory and all subdirectories to the watch list and returns the added paths
func (w *Watcher) registerRecursive(dir string, matcher *matcher) ([]string, error) {
	var dirs []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return nil
		}

		if err := shouldSkipDir(dir, path, matcher); err != nil {
			return err
		}

		addErr := w.fsWatcher.Add(path)
		if errors.Is(addErr, fsnotify.ErrClosed) {
			return addErr
		}

		if addErr != nil {
			w.log.Warn("Failed to watch directory: "+path, "error", addErr)
		} else {
			dirs = append(dirs, path)
		}

		return nil
	})

	return dirs, err
}

// shouldSkipDir returns filepath.SkipDir if the subdirectory matches an ignore pattern
func shouldSkipDir(dir, path string, matcher *matcher) error {
	if path == dir {
		return nil
	}

	relPath, _ := filepath.Rel(dir, path)

	if matcher.matchDir(relPath) {
		return filepath.SkipDir
	}

	return nil
}

// isRelevantEvent returns true if the event should trigger a reload
func isRelevantEvent(event fsnotify.Event) bool {
	return event.Has(fsnotify.Write) ||
		event.Has(fsnotify.Create) ||
		event.Has(fsnotify.Remove) ||
		event.Has(fsnotify.Rename)
}

// unregisterAll removes all tracked directories of a watched service from fsnotify and the registry
func (w *Watcher) unregisterAll(t *target) {
	for _, dir := range t.list {
		if w.unregister(dir, t.svc.ID) {
			_ = w.fsWatcher.Remove(dir)
		}
	}
}

// unregister removes a service from a directory's registry, returning true if it should be removed from fsnotify
func (w *Watcher) unregister(dir, serviceID string) bool {
	subscribers := w.registry[dir]
	if len(subscribers) == 0 {
		return true
	}

	n := 0

	for _, id := range subscribers {
		if id != serviceID {
			subscribers[n] = id
			n++
		}
	}

	if n == 0 {
		delete(w.registry, dir)
		return true
	}

	w.registry[dir] = subscribers[:n]

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
