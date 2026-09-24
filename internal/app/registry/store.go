package registry

import (
	"sync"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Store projects the bus events into the runtime read model and serves it under one lock
type Store struct {
	subscriber  contracts.Subscriber
	publisher   contracts.Publisher
	loop        *contracts.Loop
	resolved    chan struct{}
	resolveOnce sync.Once

	mu       sync.RWMutex
	snapshot *model.Snapshot
}

// NewStore creates a new runtime store
func NewStore(subscriber contracts.Subscriber, publisher contracts.Publisher) *Store {
	return &Store{
		subscriber: subscriber,
		publisher:  publisher,
		resolved:   make(chan struct{}),
		snapshot:   &model.Snapshot{},
	}
}

// update runs fn under the write lock and announces the change once the lock is released when fn reports one
func (s *Store) update(fn func(*model.Snapshot) bool) {
	if !s.commit(fn) {
		return
	}

	//nolint:errcheck // a non-critical publish never fails
	s.publisher.Publish(contracts.Message{
		Type: contracts.EventSnapshotChanged,
		Data: contracts.SnapshotChanged{},
	})
}

// commit runs fn under the write lock and returns whether it changed the snapshot
func (s *Store) commit(fn func(*model.Snapshot) bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return fn(s.snapshot)
}

// apply projects one message onto the snapshot and reports whether the visible state changed
func (s *Store) apply(snapshot *model.Snapshot, msg contracts.Message) bool {
	//nolint:exhaustive // only handling the events the read model projects
	switch msg.Type {
	case contracts.EventProfileResolved:
		return s.applyProfileResolved(snapshot, msg)
	case contracts.EventPhaseChanged:
		return applyPhaseChanged(snapshot, msg)
	case contracts.EventTierStarting:
		return applyTierStarting(snapshot, msg)
	case contracts.EventTierReady:
		return applyTierReady(snapshot, msg)
	case contracts.EventServiceStarting:
		return applyServiceStarting(snapshot, msg)
	case contracts.EventServiceReady:
		return applyServiceReady(snapshot, msg)
	case contracts.EventServiceFailed:
		return applyServiceFailed(snapshot, msg)
	case contracts.EventServiceStopping:
		return applyServiceStopping(snapshot, msg)
	case contracts.EventServiceStopped:
		return applyServiceStopped(snapshot, msg)
	case contracts.EventServiceRestarting:
		return applyServiceRestarting(snapshot, msg)
	case contracts.EventWatchStarted:
		return applyWatching(snapshot, msg, true)
	case contracts.EventWatchStopped:
		return applyWatching(snapshot, msg, false)
	case contracts.EventAPIStarted:
		return applyAPIStarted(snapshot, msg)
	case contracts.EventAPIStopped:
		return applyAPIStopped(snapshot)
	case contracts.EventServiceResourcesSampled:
		return applyServiceResourcesSampled(snapshot, msg)
	}

	return false
}

// applyProfileResolved allocates the store's own tiers and services from the payload, so no payload aliases them
func (s *Store) applyProfileResolved(snapshot *model.Snapshot, msg contracts.Message) bool {
	data, ok := msg.Data.(contracts.ProfileResolved)
	if !ok {
		return false
	}

	snapshot.Profile = data.Profile
	snapshot.Resolved = true
	snapshot.Tiers = make([]*model.Tier, len(data.Tiers))
	snapshot.Services = make(map[string]*model.Service)

	for i, tier := range data.Tiers {
		own := &model.Tier{ID: tier.ID, Name: tier.Name, Services: make([]*model.Service, len(tier.Services))}

		for j, svc := range tier.Services {
			service := *svc
			service.Status = model.StatusPending

			own.Services[j] = &service
			snapshot.Services[service.ID] = &service
		}

		snapshot.Tiers[i] = own
	}

	s.resolveOnce.Do(func() {
		close(s.resolved)
	})

	return true
}

func applyPhaseChanged(snapshot *model.Snapshot, msg contracts.Message) bool {
	data, ok := msg.Data.(contracts.PhaseChanged)
	if !ok || snapshot.Phase == data.Phase {
		return false
	}

	snapshot.Phase = data.Phase

	if data.Phase == model.PhaseStartup {
		snapshot.StartedAt = msg.Timestamp
	}

	return true
}

func applyTierStarting(snapshot *model.Snapshot, msg contracts.Message) bool {
	data, ok := msg.Data.(contracts.TierStarting)
	if !ok {
		return false
	}

	return setTierReady(snapshot, data.Name, false)
}

func applyTierReady(snapshot *model.Snapshot, msg contracts.Message) bool {
	data, ok := msg.Data.(contracts.TierReady)
	if !ok {
		return false
	}

	return setTierReady(snapshot, data.Name, true)
}

func setTierReady(snapshot *model.Snapshot, name string, ready bool) bool {
	for _, tier := range snapshot.Tiers {
		if tier.Name != name || tier.Ready == ready {
			continue
		}

		tier.Ready = ready

		return true
	}

	return false
}

func applyServiceStarting(snapshot *model.Snapshot, msg contracts.Message) bool {
	data, ok := msg.Data.(contracts.ServiceStarting)
	if !ok {
		return false
	}

	svc, ok := lifecycle(snapshot, data.Service.ID, msg)
	if !ok {
		return false
	}

	svc.Status = model.StatusStarting
	svc.Error = ""
	svc.Process = model.Process{PID: data.PID, StartedAt: data.StartedAt}
	svc.AttemptedAt = data.StartedAt

	return true
}

func applyServiceReady(snapshot *model.Snapshot, msg contracts.Message) bool {
	data, ok := msg.Data.(contracts.ServiceReady)
	if !ok {
		return false
	}

	svc, ok := lifecycle(snapshot, data.Service.ID, msg)
	if !ok {
		return false
	}

	newProcess := svc.Process.PID != data.PID || svc.Process.StartedAt != data.StartedAt

	svc.Status = model.StatusRunning
	svc.Error = ""
	svc.AttemptedAt = data.StartedAt

	if newProcess {
		svc.Process = model.Process{PID: data.PID, StartedAt: data.StartedAt}
	}

	return true
}

func applyServiceStopped(snapshot *model.Snapshot, msg contracts.Message) bool {
	data, ok := msg.Data.(contracts.ServiceStopped)
	if !ok {
		return false
	}

	svc, ok := lifecycle(snapshot, data.Service.ID, msg)
	if !ok {
		return false
	}

	svc.Status = model.StatusStopped
	svc.Process = model.Process{}
	svc.Error = ""

	return true
}

func applyServiceRestarting(snapshot *model.Snapshot, msg contracts.Message) bool {
	data, ok := msg.Data.(contracts.ServiceRestarting)
	if !ok {
		return false
	}

	svc, ok := lifecycle(snapshot, data.Service.ID, msg)
	if !ok {
		return false
	}

	svc.Status = model.StatusRestarting
	svc.Process = model.Process{PID: svc.Process.PID}

	return true
}

func applyServiceStopping(snapshot *model.Snapshot, msg contracts.Message) bool {
	data, ok := msg.Data.(contracts.ServiceStopping)
	if !ok {
		return false
	}

	svc, ok := lifecycle(snapshot, data.Service.ID, msg)
	if !ok {
		return false
	}

	svc.Status = model.StatusStopping

	return true
}

func applyServiceFailed(snapshot *model.Snapshot, msg contracts.Message) bool {
	data, ok := msg.Data.(contracts.ServiceFailed)
	if !ok {
		return false
	}

	svc, ok := lifecycle(snapshot, data.Service.ID, msg)
	if !ok {
		return false
	}

	svc.Status = model.StatusFailed
	svc.Process = model.Process{}
	svc.Error = ""

	if data.Error != nil {
		svc.Error = data.Error.Error()
	}

	return true
}

// lifecycle returns the service a lifecycle event addresses and records the event's time
func lifecycle(snapshot *model.Snapshot, id string, msg contracts.Message) (*model.Service, bool) {
	svc, exists := snapshot.Services[id]
	if !exists {
		return nil, false
	}

	svc.LifecycleAt = msg.Timestamp

	return svc, true
}

func applyWatching(snapshot *model.Snapshot, msg contracts.Message, value bool) bool {
	var id string

	switch data := msg.Data.(type) {
	case contracts.WatchStarted:
		id = data.Service.ID
	case contracts.WatchStopped:
		id = data.Service.ID
	default:
		return false
	}

	svc, exists := snapshot.Services[id]
	if !exists {
		return false
	}

	svc.Watching = value

	return true
}

func applyAPIStarted(snapshot *model.Snapshot, msg contracts.Message) bool {
	data, ok := msg.Data.(contracts.APIStarted)
	if !ok {
		return false
	}

	next := model.API{Listening: true, Address: data.Listen}
	if snapshot.API == next {
		return false
	}

	snapshot.API = next

	return true
}

func applyAPIStopped(snapshot *model.Snapshot) bool {
	if !snapshot.API.Listening {
		return false
	}

	snapshot.API = model.API{}

	return true
}

// applyServiceResourcesSampled records the usage of every sampled service that still runs the sampled PID
func applyServiceResourcesSampled(snapshot *model.Snapshot, msg contracts.Message) bool {
	data, ok := msg.Data.(contracts.ServiceResourcesSampled)
	if !ok {
		return false
	}

	changed := false

	for _, sample := range data.Services {
		svc, exists := snapshot.Services[sample.ID]
		if !exists || svc.Process.PID != sample.PID || (svc.Process.CPU == sample.CPU && svc.Process.Memory == sample.Memory) {
			continue
		}

		svc.Process.CPU = sample.CPU
		svc.Process.Memory = sample.Memory
		changed = true
	}

	return changed
}
