package registry

import (
	"sort"
	"sync"
	"time"

	"fuku/internal/app/bus"
	"fuku/internal/app/process"
	"fuku/internal/config"
)

// entry represents a registered process with its metadata
type entry struct {
	proc  process.Process
	name  string
	tier  string
	order int
}

// Lookup contains the result of a registry lookup
type Lookup struct {
	Proc     process.Process
	Name     string
	Tier     string
	Exists   bool
	Detached bool
}

// RemoveResult contains the result of removing a process from registry
type RemoveResult struct {
	Removed        bool
	Name           string
	Tier           string
	UnexpectedExit bool
}

// ProcessEntry contains a process with its service ID for shutdown ordering
type ProcessEntry struct {
	ID   string
	Proc process.Process
}

// Registry is the single source of truth for tracking running processes
type Registry interface {
	Add(tier string, svc bus.Service, proc process.Process)
	Get(id string) Lookup
	Remove(id string, proc process.Process) RemoveResult
	SnapshotReverse() []ProcessEntry
	Detach(id string)
	Wait()
}

// registry implements the Registry interface to track processes
type registry struct {
	mu        sync.Mutex
	wg        sync.WaitGroup
	processes map[string]*entry
	detached  map[string]*entry
	nextOrder int
}

// NewRegistry creates a new process registry
func NewRegistry() Registry {
	return &registry{
		processes: make(map[string]*entry),
		detached:  make(map[string]*entry),
	}
}

// Add registers a process and adds it to the WaitGroup
func (reg *registry) Add(tier string, svc bus.Service, proc process.Process) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	item := &entry{
		proc:  proc,
		name:  svc.Name,
		tier:  tier,
		order: reg.nextOrder,
	}
	reg.nextOrder++

	delete(reg.detached, svc.ID)

	reg.processes[svc.ID] = item
	reg.wg.Add(1)
}

// Get retrieves a process by ID from either active or detached processes
func (reg *registry) Get(id string) Lookup {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if item, exists := reg.processes[id]; exists {
		return Lookup{Proc: item.proc, Name: item.name, Tier: item.tier, Exists: true, Detached: false}
	}

	if item, exists := reg.detached[id]; exists {
		return Lookup{Proc: item.proc, Name: item.name, Tier: item.tier, Exists: true, Detached: true}
	}

	return Lookup{Proc: nil, Name: "", Tier: "", Exists: false, Detached: false}
}

// SnapshotReverse returns a copy of all currently tracked processes (including detached) in reverse startup order
func (reg *registry) SnapshotReverse() []ProcessEntry {
	reg.mu.Lock()

	type idEntry struct {
		id string
		e  *entry
	}

	items := make([]idEntry, 0, len(reg.processes)+len(reg.detached))
	for id, item := range reg.processes {
		items = append(items, idEntry{id: id, e: item})
	}

	for id, item := range reg.detached {
		items = append(items, idEntry{id: id, e: item})
	}

	reg.mu.Unlock()

	sort.Slice(items, func(i, j int) bool {
		return items[i].e.order > items[j].e.order
	})

	snapshot := make([]ProcessEntry, len(items))
	for i, item := range items {
		snapshot[i] = ProcessEntry{ID: item.id, Proc: item.e.proc}
	}

	return snapshot
}

// Detach removes a process from the map and marks it as detached
func (reg *registry) Detach(id string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if item, exists := reg.processes[id]; exists {
		reg.detached[id] = item
		delete(reg.processes, id)
	}
}

// Remove atomically removes a process and returns whether it was an unexpected exit
func (reg *registry) Remove(id string, proc process.Process) RemoveResult {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if item, exists := reg.detached[id]; exists && item.proc == proc {
		delete(reg.detached, id)
		reg.wg.Done()

		return RemoveResult{Removed: true, Name: item.name, Tier: item.tier, UnexpectedExit: false}
	}

	if item, exists := reg.processes[id]; exists && item.proc == proc {
		delete(reg.processes, id)
		reg.wg.Done()

		return RemoveResult{Removed: true, Name: item.name, Tier: item.tier, UnexpectedExit: true}
	}

	return RemoveResult{Removed: false, Name: "", Tier: "", UnexpectedExit: false}
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
