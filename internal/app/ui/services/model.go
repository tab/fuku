package services

import (
	"context"
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
	ID        string
	Name      string
	Tier      string
	Status    Status
	Watching  bool
	Error     error
	PID       int
	CPU       float64
	MEM       float64
	StartTime time.Time
	ReadyTime time.Time
	Blink     *components.Blink
}

// Model represents the Bubble Tea model for the services UI
type Model struct {
	ctx        context.Context
	bus        bus.Bus
	controller Controller
	store      registry.Store
	monitor    monitor.Monitor
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
		apiListen    string
		apiHealthy   bool
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

// getSelectedService returns the currently selected service state
func (m Model) getSelectedService() *ServiceState {
	if m.state.selected < 0 || m.state.selected >= len(m.state.serviceIDs) {
		return nil
	}

	return m.state.services[m.state.serviceIDs[m.state.selected]]
}

// getTotalServices returns the total count of services
func (m Model) getTotalServices() int {
	return len(m.state.serviceIDs)
}

// getReadyServices returns the count of services in ready state
func (m Model) getReadyServices() int {
	count := 0

	for _, service := range m.state.services {
		if service.Status == StatusRunning {
			count++
		}
	}

	return count
}

// calculateScrollOffset calculates the scroll offset to ensure the selected service is visible
func (m Model) calculateScrollOffset() int {
	if m.ui.servicesViewport.Height() == 0 {
		return m.ui.servicesViewport.YOffset()
	}

	lineNumber := 1
	currentIdx := 0

	for i, tier := range m.state.tiers {
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
	if len(m.state.tiers) == 0 {
		m.ui.servicesViewport.SetContent("")

		return
	}

	sections := make([]string, 0, len(m.state.tiers)+1)
	sections = append(sections, m.renderColumnHeaders())

	currentIdx := 0

	for _, tier := range m.state.tiers {
		sections = append(sections, m.renderTier(tier, &currentIdx))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	m.ui.servicesViewport.SetContent(content)
}

// refreshFromStore updates service state from the store snapshots
func (m *Model) refreshFromStore() {
	snapshots := m.store.Services()

	for _, snap := range snapshots {
		service, exists := m.state.services[snap.ID]
		if !exists {
			continue
		}

		service.Status = snap.Status
		service.PID = snap.PID
		service.CPU = snap.CPU
		service.MEM = float64(snap.Memory) / 1024 / 1024
		service.StartTime = snap.StartTime
		service.Watching = snap.Watching
	}
}
