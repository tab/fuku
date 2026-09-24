package services

import (
	"sync"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// coordination is what the guard knows about one resolved service
type coordination struct {
	service    model.Service
	dispatched bool // tier startup handed the service to the pool
	reserved   bool // a token is held by startup, an action or a watch restart
	admitted   bool // an admitted command holds the token until its handler claims it
	stopped    bool // the last admitted action stopped the service on purpose, so a file change must not restart it
}

// Guard owns the facts admission checks and the tokens that serialize work on a service
type Guard struct {
	tracker  Tracker
	mu       sync.Mutex
	phase    model.Phase
	services map[string]*coordination
}

// NewGuard creates a guard that reads live children from the tracker
func NewGuard(tracker Tracker) *Guard {
	return &Guard{tracker: tracker}
}

// open starts a run: StopAll is accepted from now on, individual actions once the profile is resolved
func (g *Guard) open() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.phase = model.PhaseStartup
	g.services = nil
}

// resolve records the resolved profile, which opens individual actions on its services
func (g *Guard) resolve(tiers []model.Tier) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.services = make(map[string]*coordination)

	for _, tier := range tiers {
		for _, svc := range tier.Services {
			g.services[svc.ID] = &coordination{service: *svc}
		}
	}
}

// halt closes admission in one step, moving a run in startup or running to stopping, and returns the phase it found
func (g *Guard) halt() model.Phase {
	g.mu.Lock()
	defer g.mu.Unlock()

	phase := g.phase

	if g.isOpen() {
		g.phase = model.PhaseStopping
	}

	return phase
}

// setPhase records the run phase admission checks against
func (g *Guard) setPhase(phase model.Phase) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.phase = phase
}

// dispatch hands a service to tier startup and takes its token in one step (a pending service is never contended)
func (g *Guard) dispatch(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	entry := g.services[id]
	entry.dispatched = true
	entry.reserved = true
	entry.stopped = false
}

// claim hands the token an admission holds to the command's handler, and reports false for a command nobody admitted
func (g *Guard) claim(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	entry, known := g.services[id]
	if !known || !entry.admitted {
		return false
	}

	entry.admitted = false

	return true
}

// release returns the service token
func (g *Guard) release(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	entry, known := g.services[id]
	if !known {
		return
	}

	entry.reserved = false
	entry.admitted = false
}

// admit validates an action against the run, the service and its token, and reserves the service when it passes
func (g *Guard) admit(id string, action contracts.Action) (Admission, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.accepting() {
		return Admission{}, contracts.ErrNotAccepting
	}

	entry, known := g.services[id]
	if !known {
		return Admission{}, contracts.ErrServiceNotFound
	}

	if !entry.dispatched {
		return Admission{}, contracts.ActionNotAllowedError{Action: action}
	}

	if entry.reserved {
		return Admission{}, contracts.ErrServiceBusy
	}

	if !allowed(action, g.live(id)) {
		return Admission{}, contracts.ActionNotAllowedError{Action: action}
	}

	entry.reserved = true
	entry.admitted = true
	entry.stopped = action == contracts.ActionStop

	return Admission{Service: entry.service, Action: action, Status: predicted[action]}, nil
}

// reserve takes the token of a dispatched service not stopped on purpose while the run accepts work, or reports false
func (g *Guard) reserve(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	entry, known := g.services[id]
	if !known || !g.accepting() || !entry.dispatched || entry.reserved || entry.stopped {
		return false
	}

	entry.reserved = true

	return true
}

// accepting reports whether actions are allowed: the run is in startup or running and the profile is resolved
func (g *Guard) accepting() bool {
	return g.isOpen() && g.services != nil
}

// isOpen reports whether the run is in startup or running
func (g *Guard) isOpen() bool {
	return g.phase == model.PhaseStartup || g.phase == model.PhaseRunning
}

// live reports whether the service has a tracked child that has not exited
func (g *Guard) live(id string) bool {
	proc, tracked := g.tracker.Get(id)

	return tracked && !exited(proc)
}

// exited reports whether a tracked child has already exited (the exit watcher untracks it a moment later)
func exited(proc contracts.Process) bool {
	select {
	case <-proc.Done():
		return true
	default:
		return false
	}
}

// allowed applies the action table: start needs no live child, stop needs one, restart needs nothing more
func allowed(action contracts.Action, live bool) bool {
	switch action {
	case contracts.ActionStart:
		return !live
	case contracts.ActionStop:
		return live
	default:
		return true
	}
}

// predicted is the status an admitted action leads to
var predicted = map[contracts.Action]model.Status{
	contracts.ActionStart:   model.StatusStarting,
	contracts.ActionStop:    model.StatusStopping,
	contracts.ActionRestart: model.StatusRestarting,
}
