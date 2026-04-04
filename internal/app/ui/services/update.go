package services

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"fuku/internal/app/bus"
	"fuku/internal/app/ui/components"
)

// Tick timing constants
const (
	tickCounterMaximum = 1000000
)

// Loader keys for system operations
const (
	loaderKeyPreflight = "_preflight"
	loaderKeyShutdown  = "_shutdown"
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
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.ui.width = msg.Width
		m.ui.height = msg.Height
		m.ui.help.SetWidth(msg.Width)

		contentWidth := msg.Width - components.PanelInnerPadding - components.RowHorizontalPadding
		m.ui.layout = components.ComputeTableLayout(contentWidth)

		panelHeight := max(msg.Height-components.PanelHeightPadding, components.MinPanelHeight)

		m.ui.servicesViewport.SetWidth(msg.Width - components.PanelInnerPadding)
		m.ui.servicesViewport.SetHeight(panelHeight - components.PanelBorderHeight)

		if !m.state.ready {
			m.state.ready = true
		}

		m.updateServicesContent()

		return m, nil

	case tea.BackgroundColorMsg:
		isDark := msg.IsDark()

		m.log.Debug().Bool("isDark", isDark).Str("color", msg.String()).Msg("TUI: Background color detected")

		m.theme = components.NewTheme(isDark)
		m.ui.help.Styles = help.DefaultStyles(isDark)
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

		if m.state.now.IsZero() || m.ui.tickCounter%components.UITicksPerSecond == 0 {
			m.state.now = time.Now()
			m.refreshFromStore()
			m.sampleAppStats()
		}

		m.updateBlinkAnimations()
		m.updateServicesContent()

		return m, tickCmd()

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
func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
		m.loader.Start(loaderKeyShutdown, "shutting down all services…")
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
		m.ui.servicesViewport.SetYOffset(m.calculateScrollOffset())
	}

	return m, nil
}

// handleDownKey moves selection down one service
func (m Model) handleDownKey() (tea.Model, tea.Cmd) {
	total := m.getTotalServices()
	if m.state.selected < total-1 {
		m.state.selected++
		m.updateServicesContent()
		m.ui.servicesViewport.SetYOffset(m.calculateScrollOffset())
	}

	return m, nil
}

// handleStopKey toggles the selected service between running and stopped
func (m Model) handleStopKey() (tea.Model, tea.Cmd) {
	service := m.getSelectedService()
	if service == nil {
		return m, nil
	}

	switch service.Status {
	case StatusStopped, StatusFailed:
		if m.controller.Start(service.ID) {
			m.loader.Start(service.ID, fmt.Sprintf("starting %s…", service.Name))

			return m, m.loader.Model.Tick
		}
	case StatusRunning:
		if m.controller.Stop(service.ID) {
			m.loader.Start(service.ID, fmt.Sprintf("stopping %s…", service.Name))

			return m, m.loader.Model.Tick
		}
	}

	return m, nil
}

// handleRestartKey restarts the selected service
func (m Model) handleRestartKey() (tea.Model, tea.Cmd) {
	service := m.getSelectedService()
	if service == nil {
		return m, nil
	}

	if m.controller.Restart(service.ID) {
		m.loader.Start(service.ID, fmt.Sprintf("restarting %s…", service.Name))

		return m, m.loader.Model.Tick
	}

	return m, nil
}

// handleMessage dispatches bus messages to specific handlers
func (m Model) handleMessage(msg bus.Message) (tea.Model, tea.Cmd) {
	//nolint:exhaustive // only handling events relevant to UI
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
	case bus.EventPreflightStarted:
		m = m.handlePreflightStarted()
	case bus.EventPreflightKill:
		m = m.handlePreflightKill(msg)
	case bus.EventPreflightComplete:
		m = m.handlePreflightComplete()
	case bus.EventAPIStarted:
		if data, ok := msg.Data.(bus.APIStarted); ok {
			m.state.apiListen = data.Listen
			m.state.apiHealthy = true
		}
	case bus.EventAPIStopped:
		m.state.apiListen = ""
		m.state.apiHealthy = false
	case bus.EventSignal:
		m = m.handleSignal()
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
	m.state.restarting = make(map[string]bool)
	m.state.serviceIDs = nil
	m.state.selected = 0
	m.loader.StopAll()

	m.state.tiers = make([]Tier, len(data.Tiers))
	m.state.tierIndex = make(map[string]int, len(data.Tiers))

	for i, tier := range data.Tiers {
		ids := make([]string, len(tier.Services))
		for j, ref := range tier.Services {
			ids[j] = ref.ID
		}

		m.log.Debug().Msgf("TUI: Adding tier %s with %d services", tier.Name, len(tier.Services))

		m.state.tiers[i] = Tier{Name: tier.Name, Services: ids, Ready: false}
		m.state.tierIndex[tier.Name] = i
		m.state.serviceIDs = append(m.state.serviceIDs, ids...)

		for _, ref := range tier.Services {
			m.state.services[ref.ID] = &ServiceState{
				ID:     ref.ID,
				Name:   ref.Name,
				Tier:   tier.Name,
				Status: StatusStarting,
				Blink:  components.NewBlink(),
			}
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

	if i, exists := m.state.tierIndex[data.Name]; exists {
		m.state.tiers[i].Ready = false
	}

	return m
}

// handleTierReady marks a tier as ready
func (m Model) handleTierReady(msg bus.Message) Model {
	data, ok := msg.Data.(bus.TierReady)
	if !ok {
		return m
	}

	if i, exists := m.state.tierIndex[data.Name]; exists {
		m.state.tiers[i].Ready = true
	}

	return m
}

// handleServiceStarting updates a service when it begins starting
func (m Model) handleServiceStarting(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ServiceStarting)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.Service.ID]; exists {
		service.Status = StatusStarting
		service.Tier = data.Tier
		service.PID = data.PID
		service.StartTime = msg.Timestamp
		service.Error = nil

		delete(m.state.restarting, data.Service.ID)

		if !m.loader.Has(data.Service.ID) {
			m.loader.Start(data.Service.ID, fmt.Sprintf("starting %s…", data.Service.Name))
		}
	}

	return m
}

// handleServiceReady updates a service when it becomes ready
func (m Model) handleServiceReady(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ServiceReady)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.Service.ID]; exists {
		service.Status = StatusRunning
		service.ReadyTime = msg.Timestamp

		m.loader.Stop(data.Service.ID)
	}

	return m
}

// handleServiceFailed updates a service when it fails
func (m Model) handleServiceFailed(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ServiceFailed)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.Service.ID]; exists {
		service.Status = StatusFailed
		service.Error = data.Error

		m.loader.Stop(data.Service.ID)
		delete(m.state.restarting, data.Service.ID)
	}

	return m
}

// handleServiceStopping updates loader when a service begins stopping
func (m Model) handleServiceStopping(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ServiceStopping)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.Service.ID]; exists {
		service.Status = StatusStopping

		if !m.loader.Has(data.Service.ID) {
			m.loader.Start(data.Service.ID, fmt.Sprintf("stopping %s…", data.Service.Name))
		}
	}

	return m
}

// handleServiceRestarting updates loader when a service begins restarting
func (m Model) handleServiceRestarting(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ServiceRestarting)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.Service.ID]; exists {
		service.Status = StatusRestarting
		m.state.restarting[data.Service.ID] = true
		m.loader.Start(data.Service.ID, fmt.Sprintf("restarting %s…", data.Service.Name))
	}

	return m
}

// handleServiceStopped stops the loader unless service is restarting
func (m Model) handleServiceStopped(msg bus.Message) Model {
	data, ok := msg.Data.(bus.ServiceStopped)
	if !ok {
		return m
	}

	service, exists := m.state.services[data.Service.ID]
	if !exists {
		return m
	}

	service.Status = StatusStopped

	if !m.state.restarting[data.Service.ID] {
		m.loader.Stop(data.Service.ID)
	}

	return m
}

// handleWatchStarted updates a service when file watching starts
func (m Model) handleWatchStarted(msg bus.Message) Model {
	data, ok := msg.Data.(bus.Service)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.ID]; exists {
		service.Watching = true
	}

	m.updateServicesContent()

	return m
}

// handleWatchStopped updates a service when file watching stops
func (m Model) handleWatchStopped(msg bus.Message) Model {
	data, ok := msg.Data.(bus.Service)
	if !ok {
		return m
	}

	if service, exists := m.state.services[data.ID]; exists {
		service.Watching = false
	}

	m.updateServicesContent()

	return m
}

// handlePreflightStarted updates loader when preflight scan begins
func (m Model) handlePreflightStarted() Model {
	m.loader.Start(loaderKeyPreflight, "preflight: scanning processes…")

	return m
}

// handlePreflightKill updates loader with the service being killed
func (m Model) handlePreflightKill(msg bus.Message) Model {
	data, ok := msg.Data.(bus.PreflightKill)
	if !ok {
		return m
	}

	m.loader.Start(loaderKeyPreflight, fmt.Sprintf("preflight: stopping %s…", data.Service))

	return m
}

// handlePreflightComplete removes preflight loader when scan finishes
func (m Model) handlePreflightComplete() Model {
	m.loader.Stop(loaderKeyPreflight)

	return m
}

// handleSignal marks the application as shutting down
func (m Model) handleSignal() Model {
	m.state.shuttingDown = true
	m.loader.Start(loaderKeyShutdown, "shutting down all services…")

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
	return tea.Tick(components.UITickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
