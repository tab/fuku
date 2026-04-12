package registry

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

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

// StatusCounts contains service counts grouped by status
type StatusCounts struct {
	Total      int
	Starting   int
	Running    int
	Stopping   int
	Restarting int
	Stopped    int
	Failed     int
}

// Store provides a bus-backed snapshot of the runtime state
type Store interface {
	Run(ctx context.Context)
	WaitReady()
	WaitResolved(ctx context.Context)
	IsResolved() bool
	Phase() string
	Profile() string
	Uptime() time.Duration
	Services() []ServiceSnapshot
	Service(id string) (ServiceSnapshot, bool)
	Counts() StatusCounts
}

// serviceState tracks the mutable state of a single service
type serviceState struct {
	id        string
	name      string
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
	bus         bus.Bus
	monitor     monitor.Monitor
	ready       chan struct{}
	resolved    chan struct{}
	resolveOnce sync.Once
	sampling    atomic.Bool

	mu           sync.RWMutex
	phase        string
	profile      string
	startTime    time.Time
	tiers        []bus.Tier
	services     map[string]*serviceState
	serviceOrder []string
	counts       StatusCounts
}

// NewStore creates a new runtime store
func NewStore(b bus.Bus, mon monitor.Monitor) Store {
	return &store{
		bus:      b,
		monitor:  mon,
		ready:    make(chan struct{}),
		resolved: make(chan struct{}),
		services: make(map[string]*serviceState),
	}
}

// WaitReady blocks until the store has subscribed to the bus
func (s *store) WaitReady() {
	<-s.ready
}

// WaitResolved blocks until the store has received the first ProfileResolved event or the context is cancelled
func (s *store) WaitResolved(ctx context.Context) {
	select {
	case <-s.resolved:
	case <-ctx.Done():
	}
}

// IsResolved returns true if the store has received profile data
func (s *store) IsResolved() bool {
	select {
	case <-s.resolved:
		return true
	default:
		return false
	}
}

// Run subscribes to the bus and maintains the runtime snapshot
func (s *store) Run(ctx context.Context) {
	msgChan := s.bus.Subscribe(ctx)
	close(s.ready)

	sampleTicker := time.NewTicker(config.StoreSampleInterval)
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
		case <-sampleTicker.C:
			if s.sampling.CompareAndSwap(false, true) {
				go s.sampleStats(ctx)
			}
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

	for _, id := range s.serviceOrder {
		svc, exists := s.services[id]
		if !exists {
			continue
		}

		snapshots = append(snapshots, s.snapshot(svc))
	}

	return snapshots
}

// Service returns a single service snapshot by ID
func (s *store) Service(id string) (ServiceSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	svc, exists := s.services[id]
	if !exists {
		return ServiceSnapshot{}, false
	}

	return s.snapshot(svc), true
}

// Counts returns the current service status counts
func (s *store) Counts() StatusCounts {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.counts
}

func (s *store) transitionStatus(svc *serviceState, newStatus Status) {
	s.decrementCount(svc.status)
	svc.status = newStatus
	s.incrementCount(newStatus)
}

func (s *store) incrementCount(status Status) {
	switch status {
	case StatusStarting:
		s.counts.Starting++
	case StatusRunning:
		s.counts.Running++
	case StatusStopping:
		s.counts.Stopping++
	case StatusRestarting:
		s.counts.Restarting++
	case StatusStopped:
		s.counts.Stopped++
	case StatusFailed:
		s.counts.Failed++
	}
}

func (s *store) decrementCount(status Status) {
	switch status {
	case StatusStarting:
		s.counts.Starting--
	case StatusRunning:
		s.counts.Running--
	case StatusStopping:
		s.counts.Stopping--
	case StatusRestarting:
		s.counts.Restarting--
	case StatusStopped:
		s.counts.Stopped--
	case StatusFailed:
		s.counts.Failed--
	}
}

func (s *store) snapshot(svc *serviceState) ServiceSnapshot {
	return ServiceSnapshot{
		ID:        svc.id,
		Name:      svc.name,
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
		s.handleProfileResolved(msg)
	case bus.EventPhaseChanged:
		s.handlePhaseChanged(msg)
	case bus.EventServiceStarting:
		s.handleServiceStarting(msg)
	case bus.EventServiceReady:
		s.setServiceStatus(msg, StatusRunning)
	case bus.EventServiceFailed:
		s.handleServiceFailed(msg)
	case bus.EventServiceStopping:
		s.setServiceStatus(msg, StatusStopping)
	case bus.EventServiceStopped:
		s.handleServiceStopped(msg)
	case bus.EventServiceRestarting:
		s.setServiceStatus(msg, StatusRestarting)
	case bus.EventWatchStarted:
		s.setWatching(msg, true)
	case bus.EventWatchStopped:
		s.setWatching(msg, false)
	}
}

func (s *store) handleProfileResolved(msg bus.Message) {
	data, ok := msg.Data.(bus.ProfileResolved)
	if !ok {
		return
	}

	s.profile = data.Profile
	s.tiers = data.Tiers
	s.initServices(data.Tiers)

	s.resolveOnce.Do(func() {
		close(s.resolved)
	})
}

func (s *store) handlePhaseChanged(msg bus.Message) {
	data, ok := msg.Data.(bus.PhaseChanged)
	if !ok {
		return
	}

	s.phase = string(data.Phase)

	if data.Phase == bus.PhaseStartup {
		s.startTime = msg.Timestamp
	}
}

func (s *store) handleServiceStarting(msg bus.Message) {
	data, ok := msg.Data.(bus.ServiceStarting)
	if !ok {
		return
	}

	svc, exists := s.services[data.Service.ID]
	if !exists {
		return
	}

	s.transitionStatus(svc, StatusStarting)
	svc.pid = data.PID
	svc.err = ""
	svc.startTime = msg.Timestamp
	svc.cpu = 0
	svc.memory = 0
}

func (s *store) handleServiceStopped(msg bus.Message) {
	data, ok := msg.Data.(bus.ServiceStopped)
	if !ok {
		return
	}

	svc, exists := s.services[data.Service.ID]
	if !exists {
		return
	}

	s.transitionStatus(svc, StatusStopped)
	svc.pid = 0
	svc.cpu = 0
	svc.memory = 0
	svc.startTime = time.Time{}
	svc.err = ""
}

// serviceIdentifier extracts the service ID from bus event data
type serviceIdentifier interface {
	ServiceID() string
}

func (s *store) setServiceStatus(msg bus.Message, status Status) {
	ident, ok := msg.Data.(serviceIdentifier)
	if !ok {
		return
	}

	if svc, exists := s.services[ident.ServiceID()]; exists {
		s.transitionStatus(svc, status)
	}
}

func (s *store) handleServiceFailed(msg bus.Message) {
	data, ok := msg.Data.(bus.ServiceFailed)
	if !ok {
		return
	}

	svc, exists := s.services[data.Service.ID]
	if !exists {
		return
	}

	s.transitionStatus(svc, StatusFailed)
	svc.pid = 0
	svc.cpu = 0
	svc.memory = 0
	svc.startTime = time.Time{}
	svc.err = ""

	if data.Error != nil {
		svc.err = data.Error.Error()
	}
}

func (s *store) setWatching(msg bus.Message, value bool) {
	data, ok := msg.Data.(bus.Service)
	if !ok {
		return
	}

	if svc, exists := s.services[data.ID]; exists {
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
	s.counts = StatusCounts{Total: totalServices, Starting: totalServices}

	tierIndex := make(map[string]int, len(tiers))
	for i, tier := range tiers {
		tierIndex[tier.Name] = i
	}

	type serviceEntry struct {
		id        string
		name      string
		tier      string
		tierOrder int
	}

	var entries []serviceEntry

	for _, tier := range tiers {
		for _, svc := range tier.Services {
			s.services[svc.ID] = &serviceState{
				id:     svc.ID,
				name:   svc.Name,
				tier:   tier.Name,
				status: StatusStarting,
			}
			entries = append(entries, serviceEntry{
				id:        svc.ID,
				name:      svc.Name,
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
		s.serviceOrder[i] = e.id
	}
}

func (s *store) sampleStats(ctx context.Context) {
	defer s.sampling.Store(false)

	s.mu.RLock()
	pids := make(map[string]int, len(s.services))

	for id, svc := range s.services {
		if svc.pid > 0 {
			pids[id] = svc.pid
		}
	}

	s.mu.RUnlock()

	if len(pids) == 0 {
		return
	}

	stats := make(map[string]monitor.Stats, len(pids))

	for id, pid := range pids {
		svcCtx, cancel := context.WithTimeout(ctx, config.StoreSampleTimeout)
		st, err := s.monitor.GetStats(svcCtx, pid)

		cancel()

		if err != nil {
			continue
		}

		stats[id] = st
	}

	s.mu.Lock()

	samples := make([]bus.ServiceMetrics, 0, len(stats))

	for id, stat := range stats {
		service, exists := s.services[id]
		if !exists || service.pid != pids[id] {
			continue
		}

		service.cpu = stat.CPU
		service.memory = stat.RawMEM

		if service.status.IsRunning() {
			samples = append(samples, bus.ServiceMetrics{
				Service: bus.Service{ID: service.id, Name: service.name},
				CPU:     stat.CPU,
				Memory:  stat.RawMEM,
			})
		}
	}

	s.mu.Unlock()

	if len(samples) > 0 {
		s.bus.Publish(bus.Message{
			Type: bus.EventServiceMetrics,
			Data: bus.ServiceMetricsBatch{Samples: samples},
		})
	}
}
