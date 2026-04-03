package registry

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"fuku/internal/app/bus"
	"fuku/internal/app/monitor"
	"fuku/internal/config"
)

// Status represents the lifecycle status of a service
type Status string

// Status values for service lifecycle
const (
	StatusStarting   Status = "starting"
	StatusRunning    Status = "running"
	StatusStopping   Status = "stopping"
	StatusRestarting Status = "restarting"
	StatusFailed     Status = "failed"
	StatusStopped    Status = "stopped"
)

// IsRunning returns true if the status is running
func (s Status) IsRunning() bool {
	return s == StatusRunning
}

// IsStartable returns true if the service can be started
func (s Status) IsStartable() bool {
	return s == StatusStopped || s == StatusFailed
}

// IsStoppable returns true if the service can be stopped
func (s Status) IsStoppable() bool {
	return s == StatusRunning
}

// IsRestartable returns true if the service can be restarted
func (s Status) IsRestartable() bool {
	return s == StatusRunning || s == StatusFailed || s == StatusStopped
}

// ServiceSnapshot contains a point-in-time snapshot of a service
type ServiceSnapshot struct {
	ID        string
	Name      string
	Tier      string
	Status    Status
	Watching  bool
	Error     string
	PID       int
	CPU       float64
	Memory    uint64
	StartTime time.Time
}

// Store provides a bus-backed snapshot of the runtime state
type Store interface {
	Run(ctx context.Context)
	WaitReady()
	Phase() string
	Profile() string
	Uptime() time.Duration
	Services() []ServiceSnapshot
	Service(name string) (ServiceSnapshot, bool)
	ServiceByID(id string) (ServiceSnapshot, bool)
}

// serviceState tracks the mutable state of a single service
type serviceState struct {
	id        string
	tier      string
	status    Status
	watching  bool
	err       string
	pid       int
	cpu       float64
	memory    uint64
	startTime time.Time
}

// store implements the Store interface
type store struct {
	cfg     *config.Config
	bus     bus.Bus
	monitor monitor.Monitor
	ready   chan struct{}

	mu           sync.RWMutex
	phase        string
	profile      string
	startTime    time.Time
	tiers        []bus.Tier
	services     map[string]*serviceState
	serviceOrder []string
}

// NewStore creates a new runtime store
func NewStore(cfg *config.Config, b bus.Bus, mon monitor.Monitor) Store {
	return &store{
		cfg:      cfg,
		bus:      b,
		monitor:  mon,
		ready:    make(chan struct{}),
		services: make(map[string]*serviceState),
	}
}

// WaitReady blocks until the store has subscribed to the bus
func (s *store) WaitReady() {
	<-s.ready
}

// Run subscribes to the bus and maintains the runtime snapshot
func (s *store) Run(ctx context.Context) {
	msgChan := s.bus.Subscribe(ctx)
	close(s.ready)

	sampleTicker := time.NewTicker(config.StoreSampleInterval)
	sampleChan := sampleTicker.C

	defer sampleTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgChan:
			if !ok {
				return
			}

			s.handleEvent(msg)
		case <-sampleChan:
			s.sampleStats(ctx)
		}
	}
}

// Phase returns the current instance phase
func (s *store) Phase() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.phase
}

// Profile returns the active profile name
func (s *store) Profile() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.profile
}

// Uptime returns the duration since the instance started (includes startup phase)
func (s *store) Uptime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.startTime.IsZero() {
		return 0
	}

	return time.Since(s.startTime)
}

// Services returns all services ordered by tier then name
func (s *store) Services() []ServiceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots := make([]ServiceSnapshot, 0, len(s.serviceOrder))

	for _, name := range s.serviceOrder {
		svc, exists := s.services[name]
		if !exists {
			continue
		}

		snapshots = append(snapshots, s.snapshot(name, svc))
	}

	return snapshots
}

// Service returns a single service snapshot
func (s *store) Service(name string) (ServiceSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	svc, exists := s.services[name]
	if !exists {
		return ServiceSnapshot{}, false
	}

	return s.snapshot(name, svc), true
}

// ServiceByID returns a single service snapshot by UUID
func (s *store) ServiceByID(id string) (ServiceSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for name, svc := range s.services {
		if svc.id == id {
			return s.snapshot(name, svc), true
		}
	}

	return ServiceSnapshot{}, false
}

func (s *store) snapshot(name string, svc *serviceState) ServiceSnapshot {
	return ServiceSnapshot{
		ID:        svc.id,
		Name:      name,
		Tier:      svc.tier,
		Status:    svc.status,
		Watching:  svc.watching,
		Error:     svc.err,
		PID:       svc.pid,
		CPU:       svc.cpu,
		Memory:    svc.memory,
		StartTime: svc.startTime,
	}
}

func (s *store) handleEvent(msg bus.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	//nolint:exhaustive // only handling lifecycle events relevant to the store
	switch msg.Type {
	case bus.EventProfileResolved:
		if data, ok := msg.Data.(bus.ProfileResolved); ok {
			s.profile = data.Profile
			s.tiers = data.Tiers
			s.initServices(data.Tiers)
		}
	case bus.EventPhaseChanged:
		if data, ok := msg.Data.(bus.PhaseChanged); ok {
			s.phase = string(data.Phase)

			if data.Phase == bus.PhaseStartup {
				s.startTime = msg.Timestamp
			}
		}
	case bus.EventServiceStarting:
		if data, ok := msg.Data.(bus.ServiceStarting); ok {
			if svc, exists := s.services[data.Service]; exists {
				svc.status = StatusStarting
				svc.pid = data.PID
				svc.err = ""
				svc.startTime = msg.Timestamp
				svc.cpu = 0
				svc.memory = 0
			}
		}
	case bus.EventServiceReady:
		if data, ok := msg.Data.(bus.ServiceReady); ok {
			if svc, exists := s.services[data.Service]; exists {
				svc.status = StatusRunning
			}
		}
	case bus.EventServiceFailed:
		s.handleServiceFailed(msg)

	case bus.EventServiceStopping:
		if data, ok := msg.Data.(bus.ServiceStopping); ok {
			if svc, exists := s.services[data.Service]; exists {
				svc.status = StatusStopping
			}
		}
	case bus.EventServiceStopped:
		if data, ok := msg.Data.(bus.ServiceStopped); ok {
			if svc, exists := s.services[data.Service]; exists {
				svc.status = StatusStopped
				svc.pid = 0
				svc.cpu = 0
				svc.memory = 0
				svc.startTime = time.Time{}
			}
		}
	case bus.EventServiceRestarting:
		if data, ok := msg.Data.(bus.ServiceRestarting); ok {
			if svc, exists := s.services[data.Service]; exists {
				svc.status = StatusRestarting
			}
		}
	case bus.EventWatchStarted:
		s.setWatching(msg, true)
	case bus.EventWatchStopped:
		s.setWatching(msg, false)
	}
}

func (s *store) handleServiceFailed(msg bus.Message) {
	data, ok := msg.Data.(bus.ServiceFailed)
	if !ok {
		return
	}

	svc, exists := s.services[data.Service]
	if !exists {
		return
	}

	svc.status = StatusFailed
	svc.pid = 0
	svc.cpu = 0
	svc.memory = 0
	svc.startTime = time.Time{}

	if data.Error != nil {
		svc.err = data.Error.Error()
	}
}

func (s *store) setWatching(msg bus.Message, value bool) {
	data, ok := msg.Data.(bus.Payload)
	if !ok {
		return
	}

	if svc, exists := s.services[data.Name]; exists {
		svc.watching = value
	}
}

func (s *store) initServices(tiers []bus.Tier) {
	totalServices := 0
	for _, tier := range tiers {
		totalServices += len(tier.Services)
	}

	s.services = make(map[string]*serviceState, totalServices)
	s.serviceOrder = nil

	tierIndex := make(map[string]int, len(tiers))
	for i, tier := range tiers {
		tierIndex[tier.Name] = i
	}

	type serviceEntry struct {
		name      string
		tier      string
		tierOrder int
	}

	var entries []serviceEntry

	for _, tier := range tiers {
		for _, name := range tier.Services {
			s.services[name] = &serviceState{
				id:     uuid.NewString(),
				tier:   tier.Name,
				status: StatusStarting,
			}
			entries = append(entries, serviceEntry{
				name:      name,
				tier:      tier.Name,
				tierOrder: tierIndex[tier.Name],
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].tierOrder != entries[j].tierOrder {
			return entries[i].tierOrder < entries[j].tierOrder
		}

		return entries[i].name < entries[j].name
	})

	s.serviceOrder = make([]string, len(entries))
	for i, e := range entries {
		s.serviceOrder[i] = e.name
	}
}

func (s *store) sampleStats(ctx context.Context) {
	s.mu.RLock()
	pids := make(map[string]int, len(s.services))

	for name, svc := range s.services {
		if svc.pid > 0 {
			pids[name] = svc.pid
		}
	}

	s.mu.RUnlock()

	if len(pids) == 0 {
		return
	}

	stats := make(map[string]monitor.Stats, len(pids))

	for name, pid := range pids {
		st, err := s.monitor.GetStats(ctx, pid)
		if err != nil {
			continue
		}

		stats[name] = st
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for name, st := range stats {
		if svc, exists := s.services[name]; exists && svc.pid == pids[name] {
			svc.cpu = st.CPU
			svc.memory = st.RawMEM
		}
	}
}
