package services

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"fuku/internal/app/bus"
	"fuku/internal/app/monitor"
	"fuku/internal/app/registry"
	"fuku/internal/app/ui/components"
	"fuku/internal/config/logger"
)

// apiHealthStatus represents the API server health state
type apiHealthStatus int

const (
	apiStatusDown  apiHealthStatus = iota // API server not started
	apiStatusReady                        // API server bound and serving
)

// Status is a type alias for registry.Status
type Status = registry.Status

// Status values re-exported from registry for convenience
const (
	StatusStarting   = registry.StatusStarting
	StatusRunning    = registry.StatusRunning
	StatusStopping   = registry.StatusStopping
	StatusRestarting = registry.StatusRestarting
	StatusFailed     = registry.StatusFailed
	StatusStopped    = registry.StatusStopped
)

// Tier represents a tier in the UI
type Tier struct {
	Name     string
	Services []string
	Ready    bool
}

// ServiceState represents the state of a service in the UI
type ServiceState struct {
	ID               string
	Name             string
	Tier             string
	Status           Status
	Watching         bool
	Error            error
	PID              int
	CPU              float64
	MEM              float64
	StartTime        time.Time
	AttemptStartedAt time.Time
	ReadyTime        time.Time
	LifecycleAt      time.Time
	LifecycleSeq     uint64
	BackfilledSeq    uint64
	StartupSampled   int
	StartupActive    bool
	WatchAt          time.Time
	WatchSeq         uint64
	Blink            *components.Blink
	Timeline         *Timeline
}

// APIListener exposes the bound API server address
type APIListener interface {
	Address() string
}

// Model represents the Bubble Tea model for the services UI
type Model struct {
	ctx        context.Context
	bus        bus.Bus
	controller Controller
	store      registry.Store
	monitor    monitor.Monitor
	api        APIListener
	loader     *Loader
	msgChan    <-chan bus.Message

	theme components.Theme

	state struct {
		profile      string
		phase        bus.Phase
		tiers        []Tier
		tierIndex    map[string]int
		services     map[string]*ServiceState
		serviceIDs   []string
		restarting   map[string]bool
		selected     int
		ready        bool
		shuttingDown bool
		appCPU       float64
		appMEM       float64
		now          time.Time
		apiStatus    apiHealthStatus

		filterQuery            string
		filterActive           bool
		filteredIDs            []string
		filteredTiers          []Tier
		preFilterSelectedID    string
		lastFilteredSelectedID string
	}

	ui struct {
		height           int
		width            int
		layout           components.TableLayout
		servicesKeys     KeyMap
		tickCounter      int
		showTips         bool
		tipOffset        int
		help             help.Model
		servicesViewport viewport.Model
	}

	log logger.Logger
}

// NewModel creates a new services UI model
func NewModel(
	ctx context.Context,
	profile string,
	b bus.Bus,
	controller Controller,
	store registry.Store,
	mon monitor.Monitor,
	apiListener APIListener,
	loader *Loader,
	log logger.Logger,
) Model {
	log = log.WithComponent("UI")
	msgChan := b.Subscribe(ctx)

	log.Debug().Msg("Created model and subscribed to events")

	isDark := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	theme := components.NewTheme(isDark)

	m := Model{
		ctx:        ctx,
		bus:        b,
		controller: controller,
		store:      store,
		monitor:    mon,
		api:        apiListener,
		loader:     loader,
		msgChan:    msgChan,
		log:        log,
		theme:      theme,
	}

	m.state.profile = profile
	m.state.phase = bus.PhaseStartup
	m.state.tiers = make([]Tier, 0)
	m.state.tierIndex = make(map[string]int)
	m.state.services = make(map[string]*ServiceState)
	m.state.restarting = make(map[string]bool)
	m.state.selected = 0
	m.state.ready = false
	m.state.shuttingDown = false

	m.ui.height = 0
	m.ui.width = 0
	m.ui.servicesKeys = DefaultKeyMap()
	m.ui.showTips = true
	//nolint:gosec // not security-critical
	m.ui.tipOffset = rand.Intn(len(components.Tips))
	m.ui.help = help.New()
	m.ui.help.Styles = help.DefaultStyles(isDark)
	m.ui.servicesViewport = viewport.New()

	return m
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loader.Model.Tick,
		waitForMsgCmd(m.msgChan),
		tickCmd(),
		requestBackgroundColorCmd,
	)
}

// requestBackgroundColorCmd asks the terminal for its background color
func requestBackgroundColorCmd() tea.Msg {
	return tea.RequestBackgroundColor()
}

// isFiltering returns true when an effective filter query is applied
func (m Model) isFiltering() bool {
	return normalizeQuery(m.state.filterQuery) != ""
}

// activeServiceIDs returns filteredIDs when filtering, otherwise serviceIDs
func (m Model) activeServiceIDs() []string {
	if m.isFiltering() {
		return m.state.filteredIDs
	}

	return m.state.serviceIDs
}

// getSelectedService returns the currently selected service state
func (m Model) getSelectedService() *ServiceState {
	ids := m.activeServiceIDs()
	if m.state.selected < 0 || m.state.selected >= len(ids) {
		return nil
	}

	return m.state.services[ids[m.state.selected]]
}

// getTotalServices returns the total count of services
func (m Model) getTotalServices() int {
	return len(m.activeServiceIDs())
}

// getReadyServices returns the count of services in ready state
func (m Model) getReadyServices() int {
	count := 0

	for _, id := range m.activeServiceIDs() {
		service, exists := m.state.services[id]
		if exists && service.Status == StatusRunning {
			count++
		}
	}

	return count
}

// getAllReadyServices returns the count of all services in ready state regardless of filter
func (m Model) getAllReadyServices() int {
	count := 0

	for _, id := range m.state.serviceIDs {
		service, exists := m.state.services[id]
		if exists && service.Status == StatusRunning {
			count++
		}
	}

	return count
}

// activeTiers returns filteredTiers when filtering, otherwise tiers
func (m Model) activeTiers() []Tier {
	if m.isFiltering() {
		return m.state.filteredTiers
	}

	return m.state.tiers
}

// calculateScrollOffset calculates the scroll offset to ensure the selected service is visible
func (m Model) calculateScrollOffset() int {
	if m.ui.servicesViewport.Height() == 0 {
		return m.ui.servicesViewport.YOffset()
	}

	lineNumber := 1
	currentIdx := 0

	for i, tier := range m.activeTiers() {
		tierStartLine := lineNumber

		if i > 0 {
			lineNumber++
		}

		lineNumber++

		serviceIndexInTier := 0

		for range tier.Services {
			if currentIdx != m.state.selected {
				lineNumber++
				currentIdx++
				serviceIndexInTier++

				continue
			}

			viewportTop := m.ui.servicesViewport.YOffset()
			viewportBottom := viewportTop + m.ui.servicesViewport.Height() - 1

			switch {
			case lineNumber < viewportTop && serviceIndexInTier == 0:
				return tierStartLine
			case lineNumber < viewportTop:
				return lineNumber
			case lineNumber > viewportBottom:
				return lineNumber - m.ui.servicesViewport.Height() + 1
			default:
				return m.ui.servicesViewport.YOffset()
			}
		}
	}

	return m.ui.servicesViewport.YOffset()
}

// updateServicesContent builds the full services content and sets it in the viewport
func (m *Model) updateServicesContent() {
	tiers := m.activeTiers()

	if len(tiers) == 0 {
		m.ui.servicesViewport.SetContent("")

		return
	}

	sections := make([]string, 0, len(tiers)+1)
	sections = append(sections, m.renderColumnHeaders())

	currentIdx := 0

	for _, tier := range tiers {
		sections = append(sections, m.renderTier(tier, &currentIdx))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	m.ui.servicesViewport.SetContent(content)
}

// refreshFromStore reconciles service state from store snapshots
func (m *Model) refreshFromStore() {
	snapshots := m.store.Services()

	for _, snap := range snapshots {
		service, exists := m.state.services[snap.ID]
		if !exists {
			continue
		}

		m.reconcileLifecycle(service, snap)

		if snap.WatchSeq >= service.WatchSeq {
			service.WatchSeq = snap.WatchSeq
			service.WatchAt = snap.WatchAt
			service.Watching = snap.Watching
		}
	}
}

// reconcileLifecycle applies a store lifecycle snapshot when it is newer or same-event healing
func (m *Model) reconcileLifecycle(service *ServiceState, snap registry.ServiceSnapshot) {
	applyLifecycle := snap.LifecycleSeq > service.LifecycleSeq ||
		(snap.LifecycleSeq == service.LifecycleSeq && snap.Status == service.Status)

	if !applyLifecycle {
		return
	}

	stateAdvanced := snap.LifecycleSeq > service.LifecycleSeq || snap.Status != service.Status
	newProcess := service.AttemptStartedAt != snap.AttemptStartedAt

	if newProcess {
		service.StartupSampled = 0
	}

	if snap.Status == registry.StatusStarting {
		service.StartupActive = true
	}

	//nolint:exhaustive // only running and terminal states need startup backfill
	switch snap.Status {
	case registry.StatusRunning:
		backfillStartupHistory(service, snap.LifecycleSeq, snap.AttemptStartedAt, snap.LifecycleAt)
		service.StartupActive = false
	case registry.StatusFailed, registry.StatusStopped:
		switch {
		case !snap.AttemptStartedAt.IsZero() && snap.AttemptStartedAt != service.AttemptStartedAt:
			backfillStartupHistory(service, snap.LifecycleSeq, snap.AttemptStartedAt, snap.LifecycleAt)
		case service.StartupActive && !service.AttemptStartedAt.IsZero():
			backfillStartupHistory(service, snap.LifecycleSeq, service.AttemptStartedAt, snap.LifecycleAt)
		}

		service.StartupActive = false
	}

	service.LifecycleSeq = snap.LifecycleSeq
	service.LifecycleAt = snap.LifecycleAt
	service.Status = snap.Status
	service.PID = snap.PID
	service.AttemptStartedAt = snap.AttemptStartedAt

	if snap.Status == registry.StatusRestarting {
		service.StartTime = time.Time{}
	} else {
		service.StartTime = snap.StartTime
	}

	switch {
	case newProcess:
		service.CPU = 0
		service.MEM = 0
	default:
		service.CPU = snap.CPU
		service.MEM = float64(snap.Memory) / 1024 / 1024
	}

	switch {
	case snap.Error != "":
		service.Error = errors.New(snap.Error)
	default:
		service.Error = nil
	}

	if stateAdvanced {
		m.reconcileLifecycleEffects(service, snap.Status)
	}
}

// reconcileLifecycleEffects syncs loader and restarting state with the reconciled status
func (m *Model) reconcileLifecycleEffects(service *ServiceState, status registry.Status) {
	//nolint:exhaustive // only terminal and restarting states need effect reconciliation
	switch status {
	case registry.StatusRunning, registry.StatusFailed:
		delete(m.state.restarting, service.ID)
		m.loader.Stop(service.ID)
	case registry.StatusStopped:
		if !m.state.restarting[service.ID] {
			m.loader.Stop(service.ID)
		}
	case registry.StatusRestarting:
		m.state.restarting[service.ID] = true
	}
}
