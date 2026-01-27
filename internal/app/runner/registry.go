package runner

import (
	"sort"
	"sync"
	"time"

	"fuku/internal/config"
)

// entry represents a registered process with its metadata
type entry struct {
	proc  Process
	tier  string
	order int
}

// Lookup contains the result of a registry lookup
type Lookup struct {
	Proc     Process
	Tier     string
	Exists   bool
	Detached bool
}

// RemoveResult contains the result of removing a process from registry
type RemoveResult struct {
	Removed        bool
	Tier           string
	UnexpectedExit bool
}

// Registry is the single source of truth for tracking running processes
type Registry interface {
	Add(name string, proc Process, tier string)
	Get(name string) Lookup
	Remove(name string, proc Process) RemoveResult
	SnapshotReverse() []Process
	Detach(name string)
	Wait()
	MarkRestarting(name string)
	IsRestarting(name string) bool
	ClearRestarting(name string)
}

// registry implements the Registry interface to track processes
type registry struct {
	mu         sync.Mutex
	wg         sync.WaitGroup
	processes  map[string]*entry
	detached   map[string]*entry
	restarting map[string]bool
	nextOrder  int
}

// NewRegistry creates a new process registry
func NewRegistry() Registry {
	return &registry{
		processes:  make(map[string]*entry),
		detached:   make(map[string]*entry),
		restarting: make(map[string]bool),
	}
}

// Add registers a process and adds it to the WaitGroup
func (reg *registry) Add(name string, proc Process, tier string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	item := &entry{
		proc:  proc,
		tier:  tier,
		order: reg.nextOrder,
	}
	reg.nextOrder++

	delete(reg.detached, name)

	reg.processes[name] = item
	reg.wg.Add(1)
}

// Get retrieves a process by name from either active or detached processes
func (reg *registry) Get(name string) Lookup {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if item, exists := reg.processes[name]; exists {
		return Lookup{Proc: item.proc, Tier: item.tier, Exists: true, Detached: false}
	}

	if item, exists := reg.detached[name]; exists {
		return Lookup{Proc: item.proc, Tier: item.tier, Exists: true, Detached: true}
	}

	return Lookup{Proc: nil, Tier: "", Exists: false, Detached: false}
}

// SnapshotReverse returns a copy of all currently tracked processes (including detached) in reverse startup order
func (reg *registry) SnapshotReverse() []Process {
	reg.mu.Lock()

	entries := make([]*entry, 0, len(reg.processes)+len(reg.detached))
	for _, item := range reg.processes {
		entries = append(entries, item)
	}

	for _, item := range reg.detached {
		entries = append(entries, item)
	}

	reg.mu.Unlock()

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].order > entries[j].order
	})

	snapshot := make([]Process, len(entries))
	for i, item := range entries {
		snapshot[i] = item.proc
	}

	return snapshot
}

// Detach removes a process from the map and marks it as detached
func (reg *registry) Detach(name string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if item, exists := reg.processes[name]; exists {
		reg.detached[name] = item
		delete(reg.processes, name)
	}
}

// Remove atomically removes a process and returns whether it was an unexpected exit
func (reg *registry) Remove(name string, proc Process) RemoveResult {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if item, exists := reg.detached[name]; exists && item.proc == proc {
		delete(reg.detached, name)
		reg.wg.Done()

		return RemoveResult{Removed: true, Tier: item.tier, UnexpectedExit: false}
	}

	if item, exists := reg.processes[name]; exists && item.proc == proc {
		delete(reg.processes, name)
		reg.wg.Done()

		return RemoveResult{Removed: true, Tier: item.tier, UnexpectedExit: true}
	}

	return RemoveResult{Removed: false, Tier: "", UnexpectedExit: false}
}

// Wait blocks until all tracked processes have finished
func (reg *registry) Wait() {
	done := make(chan struct{})

	go func() {
		reg.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(config.ShutdownTimeout):
		return
	}
}

// MarkRestarting marks a service as having a restart in progress
func (reg *registry) MarkRestarting(name string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	reg.restarting[name] = true
}

// IsRestarting returns true if a service has a restart in progress
func (reg *registry) IsRestarting(name string) bool {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	return reg.restarting[name]
}

// ClearRestarting clears the restarting flag for a service
func (reg *registry) ClearRestarting(name string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	delete(reg.restarting, name)
}
