package services

import (
	"context"
	"math/rand"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/looplab/fsm"

	"fuku/internal/app/bus"
	"fuku/internal/app/monitor"
	"fuku/internal/app/ui/components"
	"fuku/internal/config/logger"
)

// Status represents the status of a service
type Status string

// Status values for service lifecycle
const (
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopping Status = "stopping"
	StatusFailed   Status = "failed"
	StatusStopped  Status = "stopped"
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
	Name     string
	Tier     string
	Status   Status
	Watching bool
	Error    error
	FSM      *fsm.FSM
	Monitor  ServiceMonitor
	Blink    *components.Blink
}

// MarkStarting sets the service status to starting and clears any previous error
func (s *ServiceState) MarkStarting() {
	s.Status = StatusStarting
	s.Error = nil
}

// MarkRunning sets the service status to ready (running)
func (s *ServiceState) MarkRunning() {
	s.Status = StatusRunning
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

// IsNil returns true if the service or its FSM is nil (handles nil receiver)
func (s *ServiceState) IsNil() bool {
	return s == nil || s.FSM == nil
}

// Model represents the Bubble Tea model for the services UI
type Model struct {
	ctx        context.Context
	bus        bus.Bus
	controller Controller
	monitor    monitor.Monitor
	loader     *Loader
	msgChan    <-chan bus.Message

	state struct {
		profile      string
		phase        bus.Phase
		tiers        []Tier
		services     map[string]*ServiceState
		selected     int
		ready        bool
		shuttingDown bool
		appCPU       float64
		appMEM       float64
	}

	ui struct {
		height           int
		width            int
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
	monitor monitor.Monitor,
	loader *Loader,
	log logger.Logger,
) Model {
	log = log.WithComponent("UI")
	msgChan := b.Subscribe(ctx)

	log.Debug().Msg("Created model and subscribed to events")

	m := Model{
		ctx:        ctx,
		bus:        b,
		controller: controller,
		monitor:    monitor,
		loader:     loader,
		msgChan:    msgChan,
		log:        log,
	}

	m.state.profile = profile
	m.state.phase = bus.PhaseStartup
	m.state.tiers = make([]Tier, 0)
	m.state.services = make(map[string]*ServiceState)
	m.state.selected = 0
	m.state.ready = false
	m.state.shuttingDown = false

	m.ui.height = 0
	m.ui.width = 0
	m.ui.servicesKeys = DefaultKeyMap()
	m.ui.showTips = true
	m.ui.tipOffset = rand.Intn(len(components.Tips)) //nolint:gosec // not security-critical
	m.ui.help = help.New()
	m.ui.servicesViewport = viewport.New(0, 0)

	return m
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loader.Model.Tick,
		waitForMsgCmd(m.msgChan),
		tickCmd(),
		statsWorkerCmd(m.ctx, &m),
	)
}

// getSelectedService returns the currently selected service state
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

// getTotalServices returns the total count of services
func (m Model) getTotalServices() int {
	total := 0
	for _, tier := range m.state.tiers {
		total += len(tier.Services)
	}

	return total
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

// getMaxServiceNameLength returns the maximum service name length for formatting
func (m Model) getMaxServiceNameLength() int {
	maxLen := components.ServiceNameMinWidth

	for _, service := range m.state.services {
		nameWidth := lipgloss.Width(service.Name)
		if nameWidth > maxLen {
			maxLen = nameWidth
		}
	}

	return maxLen
}

// calculateScrollOffset calculates the scroll offset to ensure the selected service is visible
func (m Model) calculateScrollOffset() int {
	if m.ui.servicesViewport.Height == 0 {
		return m.ui.servicesViewport.YOffset
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

// updateServicesContent builds the full services content and sets it in the viewport
func (m *Model) updateServicesContent() {
	if len(m.state.tiers) == 0 {
		m.ui.servicesViewport.SetContent("")

		return
	}

	maxNameLen := m.getMaxServiceNameLength()

	sections := make([]string, 0, len(m.state.tiers)+1)
	sections = append(sections, m.renderColumnHeaders(maxNameLen))

	currentIdx := 0

	for _, tier := range m.state.tiers {
		sections = append(sections, m.renderTier(tier, &currentIdx, maxNameLen))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	m.ui.servicesViewport.SetContent(content)
}
