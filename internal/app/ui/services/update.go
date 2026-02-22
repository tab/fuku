package services

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"fuku/internal/app/bus"
	"fuku/internal/app/ui/components"
)

// Tick timing constants
const (
	tickInterval       = components.UITickInterval
	tickCounterMaximum = 1000000
)

// msgMsg wraps a bus message for tea messaging
type msgMsg bus.Message

// tickMsg signals a UI tick for animations
type tickMsg time.Time

// channelClosedMsg signals the event channel has closed
type channelClosedMsg struct{}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.ui.width = msg.Width
		m.ui.height = msg.Height
		m.ui.help.Width = msg.Width

		panelHeight := msg.Height - components.PanelHeightPadding
		if panelHeight < components.MinPanelHeight {
			panelHeight = components.MinPanelHeight
		}

		m.ui.servicesViewport.Width = msg.Width - components.PanelInnerPadding
		m.ui.servicesViewport.Height = panelHeight - components.PanelBorderHeight

		if !m.state.ready {
			m.state.ready = true
		}

		m.updateServicesContent()

		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd

		m.loader.Model, cmd = m.loader.Model.Update(msg)

		return m, cmd

	case tickMsg:
		m.ui.tickCounter++

		if m.ui.tickCounter >= tickCounterMaximum {
			m.ui.tickCounter = 0
		}

		hasActiveBlinking := m.updateBlinkAnimations()
		if hasActiveBlinking {
			m.updateServicesContent()
		}

		return m, tickCmd()

	case statsUpdateMsg:
		m.applyStatsUpdate(msg)
		m.updateServicesContent()

		return m, statsWorkerCmd(m.ctx, &m)

	case msgMsg:
		return m.handleMessage(bus.Message(msg))

	case channelClosedMsg:
		m.log.Warn().Msg("TUI: Event channel closed, quitting")
		m.loader.StopAll()

		return m, tea.Quit
	}

	return m, nil
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.ui.servicesKeys.ForceQuit) {
		m.log.Warn().Msg("TUI: Force quit requested, exiting immediately")
		m.loader.StopAll()

		return m, tea.Quit
	}

	if m.state.shuttingDown {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.ui.servicesKeys.Quit):
		m.state.shuttingDown = true
		m.loader.Start("_shutdown", "shutting down all services…")
		m.controller.StopAll()

		return m, waitForMsgCmd(m.msgChan)

	case key.Matches(msg, m.ui.servicesKeys.Up):
		return m.handleUpKey()

	case key.Matches(msg, m.ui.servicesKeys.Down):
		return m.handleDownKey()

	case key.Matches(msg, m.ui.servicesKeys.Stop):
		return m.handleStopKey()

	case key.Matches(msg, m.ui.servicesKeys.Restart):
		return m.handleRestartKey()

	case key.Matches(msg, m.ui.servicesKeys.ToggleTips):
		m.ui.showTips = !m.ui.showTips
		return m, nil
	}

	switch msg.String() {
	case "pgup", "pgdown", "home", "end":
		var cmd tea.Cmd

		m.ui.servicesViewport, cmd = m.ui.servicesViewport.Update(msg)

		return m, cmd
	}

	return m, nil
}

// handleUpKey moves selection up one service
func (m Model) handleUpKey() (tea.Model, tea.Cmd) {
	if m.state.selected > 0 {
		m.state.selected--
		m.updateServicesContent()
		m.ui.servicesViewport.YOffset = m.calculateScrollOffset()
	}

	return m, nil
}

// handleDownKey moves selection down one service
func (m Model) handleDownKey() (tea.Model, tea.Cmd) {
	total := m.getTotalServices()
	if m.state.selected < total-1 {
		m.state.selected++
		m.updateServicesContent()
		m.ui.servicesViewport.YOffset = m.calculateScrollOffset()
	}

	return m, nil
}

// handleStopKey toggles the selected service between running and stopped
func (m Model) handleStopKey() (tea.Model, tea.Cmd) {
	service := m.getSelectedService()
	if service.IsNil() {
		return m, nil
	}

	if service.FSM.Current() == Stopped {
		m.controller.Start(m.ctx, service)
		return m, m.loader.Model.Tick
	}

	if service.FSM.Current() == Running {
		m.controller.Stop(m.ctx, service)
		return m, m.loader.Model.Tick
	}

	return m, nil
}

// handleRestartKey restarts the selected service
func (m Model) handleRestartKey() (tea.Model, tea.Cmd) {
	service := m.getSelectedService()
	if service.IsNil() {
		return m, nil
	}

	m.controller.Restart(m.ctx, service)

	return m, m.loader.Model.Tick
}

// handleMessage dispatches bus messages to specific handlers
func (m Model) handleMessage(msg bus.Message) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case bus.EventProfileResolved:
		m = m.handleProfileResolved(msg)
	case bus.EventPhaseChanged:
		return m.handlePhaseChanged(msg)
	case bus.EventTierStarting:
		m = m.handleTierStarting(msg)
	case bus.EventTierReady:
		m = m.handleTierReady(msg)
	case bus.EventServiceStarting:
		m = m.handleServiceStarting(msg)
	case bus.EventServiceReady:
		m = m.handleServiceReady(msg)
	case bus.EventServiceFailed:
		m = m.handleServiceFailed(msg)
	case bus.EventServiceStopping:
		m = m.handleServiceStopping(msg)
	case bus.EventServiceStopped:
		m = m.handleServiceStopped(msg)
	case bus.EventServiceRestarting:
		m = m.handleServiceRestarting(msg)
	case bus.EventWatchStarted:
		m = m.handleWatchStarted(msg)
	case bus.EventWatchStopped:
		m = m.handleWatchStopped(msg)
	case bus.EventSignal:
		m.state.shuttingDown = true
		m.loader.Start("_shutdown", "shutting down all services…")
	}

	return m, waitForMsgCmd(m.msgChan)
}

// handleProfileResolved initializes services from profile data
func (m Model) handleProfileResolved(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ProfileResolved)
	if !ok {
		m.log.Error().Msg("TUI: Failed to cast ProfileResolvedData")
		return m
	}

	m.log.Debug().Msgf("TUI: ProfileResolved - profile=%s, tiers=%d", data.Profile, len(data.Tiers))

	m.state.services = make(map[string]*ServiceState)
	m.state.selected = 0
	m.loader.StopAll()

	m.state.tiers = make([]Tier, len(data.Tiers))
	for i, tier := range data.Tiers {
		m.log.Debug().Msgf("TUI: Adding tier %s with %d services: %v", tier.Name, len(tier.Services), tier.Services)

		m.state.tiers[i] = Tier{Name: tier.Name, Services: tier.Services, Ready: false}
		for _, serviceName := range tier.Services {
			service := &ServiceState{
				Name:   serviceName,
				Tier:   tier.Name,
				Status: StatusStarting,
				Blink:  components.NewBlink(),
			}
			service.FSM = newServiceFSM(service, m.loader, m.log)
			m.state.services[serviceName] = service
		}
	}

	m.log.Debug().Msgf("TUI: After ProfileResolved - tiers=%d, services=%d", len(m.state.tiers), len(m.state.services))

	return m
}

// handlePhaseChanged updates the application phase state
func (m Model) handlePhaseChanged(msg bus.Message) (Model, tea.Cmd) {
	data, ok := msg.Data.(bus.PhaseChanged)
	if !ok {
		return m, waitForMsgCmd(m.msgChan)
	}

	m.state.phase = data.Phase
	if m.state.phase == bus.PhaseStopped {
		m.loader.StopAll()

		return m, tea.Quit
	}

	return m, waitForMsgCmd(m.msgChan)
}

// handleTierStarting marks a tier as not ready when starting
func (m Model) handleTierStarting(msg bus.Message) Model {
	data, ok := msg.Data.(bus.TierStarting)
	if !ok {
		return m
	}

	for i, tier := range m.state.tiers {
		if tier.Name == data.Name {
			m.state.tiers[i].Ready = false
			break
		}
	}

	return m
}

// handleTierReady marks a tier as ready
func (m Model) handleTierReady(msg bus.Message) Model {
	data, ok := msg.Data.(bus.Payload)
	if !ok {
		return m
	}

	for i, tier := range m.state.tiers {
		if tier.Name == data.Name {
			m.state.tiers[i].Ready = true
			break
		}
	}

	return m
}

// handleServiceStarting updates a service when it begins starting
func (m Model) handleServiceStarting(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ServiceStarting)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.Service]; exists {
		service.Monitor.StartTime = msg.Timestamp
		service.Tier = data.Tier
		m.controller.HandleStarting(m.ctx, service, data.PID)
	}

	return m
}

// handleServiceReady updates a service when it becomes ready
func (m Model) handleServiceReady(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ServiceReady)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.Service]; exists {
		service.Monitor.ReadyTime = msg.Timestamp

		m.loader.Stop(data.Service)
		m.controller.HandleReady(m.ctx, service)
	}

	return m
}

// handleServiceFailed updates a service when it fails
func (m Model) handleServiceFailed(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ServiceFailed)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.Service]; exists {
		service.Error = data.Error
		m.loader.Stop(data.Service)
		m.controller.HandleFailed(m.ctx, service)
	}

	return m
}

// handleServiceStopping updates a service when it begins stopping
func (m Model) handleServiceStopping(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ServiceStopping)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.Service]; exists {
		m.controller.HandleStopping(m.ctx, service)
	}

	return m
}

// handleServiceRestarting updates a service when it begins restarting
func (m Model) handleServiceRestarting(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ServiceRestarting)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.Service]; exists {
		m.controller.HandleRestarting(m.ctx, service)
	}

	return m
}

// handleServiceStopped updates a service when it stops
func (m Model) handleServiceStopped(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ServiceStopped)
	if !ok {
		return m
	}

	service, exists := m.state.services[data.Service]
	if !exists {
		return m
	}

	wasRestarting := m.controller.HandleStopped(m.ctx, service)
	if !wasRestarting {
		m.loader.Stop(data.Service)
	}

	return m
}

// handleWatchStarted updates a service when file watching starts
func (m Model) handleWatchStarted(msg bus.Message) Model {
	data, ok := msg.Data.(bus.Payload)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.Name]; exists {
		service.Watching = true
	}

	m.updateServicesContent()

	return m
}

// handleWatchStopped updates a service when file watching stops
func (m Model) handleWatchStopped(msg bus.Message) Model {
	data, ok := msg.Data.(bus.Payload)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.Name]; exists {
		service.Watching = false
	}

	m.updateServicesContent()

	return m
}

// waitForMsgCmd returns a command that waits for the next message
func waitForMsgCmd(msgChan <-chan bus.Message) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-msgChan
		if !ok {
			return channelClosedMsg{}
		}

		return msgMsg(msg)
	}
}

// tickCmd returns a command that sends a tick after the interval
func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
