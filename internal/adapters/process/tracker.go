package process

import (
	"sort"
	"sync"

	"fuku/internal/contracts"
)

// entry is one tracked child with its start position
type entry struct {
	proc  contracts.Process
	order int
}

// Tracker holds the live child of every service in start order
type Tracker struct {
	mu        sync.Mutex
	active    map[string]*entry
	detached  map[string]*entry
	nextOrder int
}

// NewTracker creates an empty process tracker
func NewTracker() *Tracker {
	return &Tracker{
		active:   make(map[string]*entry),
		detached: make(map[string]*entry),
	}
}

// track runs start under the lock and records its handle by service ID, so no live child is ever untracked
func (t *Tracker) track(start func() (*Handle, error)) (*Handle, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	handle, err := start()
	if err != nil {
		return nil, err
	}

	id := handle.svc.ID

	delete(t.detached, id)

	t.active[id] = &entry{proc: handle, order: t.nextOrder}
	t.nextOrder++

	return handle, nil
}

// Get returns the child tracked for a service, active or detached
func (t *Tracker) Get(id string) (contracts.Process, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if item, exists := t.active[id]; exists {
		return item.proc, true
	}

	if item, exists := t.detached[id]; exists {
		return item.proc, true
	}

	return nil, false
}

// Detach marks the child as being stopped on purpose, so its exit is expected
func (t *Tracker) Detach(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if item, exists := t.active[id]; exists {
		t.detached[id] = item
		delete(t.active, id)
	}
}

// Untrack forgets an exited child and reports whether its exit was unrequested (a detached or unknown handle is not)
func (t *Tracker) Untrack(id string, proc contracts.Process) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if item, exists := t.detached[id]; exists && item.proc == proc {
		delete(t.detached, id)

		return false
	}

	if item, exists := t.active[id]; exists && item.proc == proc {
		delete(t.active, id)

		return true
	}

	return false
}

// Reverse lists every tracked child, detached included, newest first
func (t *Tracker) Reverse() []contracts.Process {
	t.mu.Lock()

	items := make([]*entry, 0, len(t.active)+len(t.detached))
	for _, item := range t.active {
		items = append(items, item)
	}

	for _, item := range t.detached {
		items = append(items, item)
	}

	t.mu.Unlock()

	sort.Slice(items, func(i, j int) bool {
		return items[i].order > items[j].order
	})

	procs := make([]contracts.Process, len(items))
	for i, item := range items {
		procs[i] = item.proc
	}

	return procs
}
