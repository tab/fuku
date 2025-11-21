package services

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/looplab/fsm"

	"fuku/internal/app/monitor"
	"fuku/internal/app/runtime"
	"fuku/internal/app/ui"
	"fuku/internal/app/ui/components"
	"fuku/internal/app/ui/navigation"
	"fuku/internal/config/logger"
)

// subscriber interface for starting log event subscription
type subscriber interface {
	StartCmd(ctx context.Context) tea.Cmd
}

// Status represents the status of a service
type Status string

const (
	StatusStarting Status = "Starting"
	StatusReady    Status = "Ready"
	StatusStopping Status = "Stopping"
	StatusFailed   Status = "Failed"
	StatusStopped  Status = "Stopped"
)

// Tier represents a tier in the UI
type Tier struct {
	Name     string
	Services []string
	Ready    bool
}

// ServiceMonitor contains runtime monitoring data for a service
type ServiceMonitor struct {
	PID       int
	CPU       float64
	MEM       float64
	StartTime time.Time
	ReadyTime time.Time
}

// ServiceState represents the state of a service
type ServiceState struct {
	Name    string
	Tier    string
	Status  Status
	Error   error
	FSM     *fsm.FSM
	Monitor ServiceMonitor
	Blink   *components.Blink
}

// MarkStarting sets the service status to starting
func (s *ServiceState) MarkStarting() {
	s.Status = StatusStarting
}

// MarkRunning sets the service status to ready (running)
func (s *ServiceState) MarkRunning() {
	s.Status = StatusReady
}

// MarkStopping sets the service status to stopping
func (s *ServiceState) MarkStopping() {
	s.Status = StatusStopping
}

// MarkStopped sets the service status to stopped and clears PID
func (s *ServiceState) MarkStopped() {
	s.Status = StatusStopped
	s.Monitor.PID = 0
}

// MarkFailed sets the service status to failed
func (s *ServiceState) MarkFailed() {
	s.Status = StatusFailed
}

// Model represents the Bubble Tea model for the services UI
type Model struct {
	ctx        context.Context
	event      runtime.EventBus
	command    runtime.CommandBus
	controller Controller
	monitor    monitor.Monitor
	logView    ui.LogView
	navigator  navigation.Navigator
	loader     *Loader
	subscriber subscriber
	eventChan  <-chan runtime.Event

	state struct {
		profile      string
		phase        runtime.Phase
		tiers        []Tier
		services     map[string]*ServiceState
		selected     int
		ready        bool
		shuttingDown bool
	}

	ui struct {
		width            int
		height           int
		keys             KeyMap
		help             help.Model
		servicesViewport viewport.Model
		tickCounter      int
	}

	log logger.Logger
}

// NewModel creates a new services UI model
func NewModel(
	ctx context.Context,
	profile string,
	event runtime.EventBus,
	command runtime.CommandBus,
	controller Controller,
	monitor monitor.Monitor,
	logView ui.LogView,
	navigator navigation.Navigator,
	loader *Loader,
	sub subscriber,
	log logger.Logger,
) Model {
	eventChan := event.Subscribe(ctx)

	log.Debug().Msg("TUI: Created model and subscribed to events")

	m := Model{
		ctx:        ctx,
		event:      event,
		command:    command,
		controller: controller,
		monitor:    monitor,
		loader:     loader,
		subscriber: sub,
		eventChan:  eventChan,
		logView:    logView,
		navigator:  navigator,
		log:        log,
	}

	m.state.profile = profile
	m.state.phase = runtime.PhaseStartup
	m.state.tiers = make([]Tier, 0)
	m.state.services = make(map[string]*ServiceState)
	m.state.selected = 0
	m.state.ready = false
	m.state.shuttingDown = false

	m.ui.width = 0
	m.ui.height = 0
	m.ui.keys = DefaultKeyMap()
	m.ui.help = help.New()
	m.ui.servicesViewport = viewport.New(0, 0)

	return m
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.loader.Model.Tick,
		waitForEventCmd(m.eventChan),
		tickCmd(),
	}

	if m.subscriber != nil {
		cmds = append(cmds, m.subscriber.StartCmd(m.ctx))
	}

	return tea.Batch(cmds...)
}

func (m Model) getSelectedService() *ServiceState {
	if m.state.selected < 0 {
		return nil
	}

	idx := 0

	for _, tier := range m.state.tiers {
		for _, serviceName := range tier.Services {
			if idx == m.state.selected {
				return m.state.services[serviceName]
			}

			idx++
		}
	}

	return nil
}

func (m Model) getTotalServices() int {
	total := 0
	for _, tier := range m.state.tiers {
		total += len(tier.Services)
	}

	return total
}

func (m Model) getReadyServices() int {
	count := 0

	for _, service := range m.state.services {
		if service.Status == StatusReady {
			count++
		}
	}

	return count
}

func (m Model) getMaxServiceNameLength() int {
	maxLen := 20

	for _, service := range m.state.services {
		if len(service.Name) > maxLen {
			maxLen = len(service.Name)
		}
	}

	return maxLen
}

func (m Model) calculateScrollOffset() int {
	if m.ui.servicesViewport.Height == 0 {
		return m.ui.servicesViewport.YOffset
	}

	lineNumber := 0
	currentIdx := 0

	for i, tier := range m.state.tiers {
		tierStartLine := lineNumber

		if i > 0 {
			lineNumber++
		}

		lineNumber++

		serviceIndexInTier := 0

		for range tier.Services {
			if currentIdx == m.state.selected {
				viewportTop := m.ui.servicesViewport.YOffset
				viewportBottom := viewportTop + m.ui.servicesViewport.Height - 1

				if lineNumber < viewportTop {
					if serviceIndexInTier == 0 {
						return tierStartLine
					}

					return lineNumber
				} else if lineNumber > viewportBottom {
					return lineNumber - m.ui.servicesViewport.Height + 1
				}

				return m.ui.servicesViewport.YOffset
			}

			lineNumber++
			currentIdx++
			serviceIndexInTier++
		}
	}

	return m.ui.servicesViewport.YOffset
}
