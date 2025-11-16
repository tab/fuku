package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"fuku/internal/app/runtime"
)

const (
	minLogMessageWidth      = 20
	maxLogServiceNameLength = 15
)

type eventMsg runtime.Event

type tickMsg time.Time

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

		panelHeight := msg.Height - panelHeightOffset
		if panelHeight < minPanelHeight {
			panelHeight = minPanelHeight
		}

		m.servicesViewport.Width = msg.Width - viewportWidthPadding
		m.servicesViewport.Height = panelHeight
		m.logsViewport.Width = msg.Width - viewportWidthPadding
		m.logsViewport.Height = panelHeight

		if !m.ready {
			m.ready = true
		}

		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd

		m.loader.Model, cmd = m.loader.Model.Update(msg)

		return m, cmd

	case tickMsg:
		m.updateProcessStats()
		return m, tickCmd()

	case eventMsg:
		return m.handleEvent(runtime.Event(msg))
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		m.quitting = true
		m.loader.Start("_shutdown", "Shutting down all services…")
		m.controller.StopAll()

		return m, tea.Quit

	case key.Matches(msg, m.keys.ForceQuit):
		m.quitting = true
		m.controller.StopAll()

		return m, tea.Quit

	case key.Matches(msg, m.keys.ToggleLogs):
		if m.viewMode == ViewModeServices {
			m.viewMode = ViewModeLogs
		} else {
			m.viewMode = ViewModeServices
		}

		return m, nil

	case key.Matches(msg, m.keys.Autoscroll):
		return m.handleAutoscroll()

	case key.Matches(msg, m.keys.Up):
		return m.handleUpKey(msg)

	case key.Matches(msg, m.keys.Down):
		return m.handleDownKey(msg)

	case key.Matches(msg, m.keys.Stop):
		return m.handleStopKey()

	case key.Matches(msg, m.keys.Restart):
		return m.handleRestartKey()

	case key.Matches(msg, m.keys.ToggleLogSub):
		return m.handleToggleLogSub()
	}

	switch msg.String() {
	case "pgup", "pgdown", "home", "end":
		var cmd tea.Cmd
		if m.viewMode == ViewModeLogs {
			m.logsViewport, cmd = m.logsViewport.Update(msg)
		} else {
			m.servicesViewport, cmd = m.servicesViewport.Update(msg)
		}

		return m, cmd
	}

	return m, nil
}

func (m Model) handleAutoscroll() (tea.Model, tea.Cmd) {
	if m.viewMode == ViewModeLogs {
		m.autoscroll = !m.autoscroll
		if m.autoscroll {
			m.logsViewport.GotoBottom()
		}
	}

	return m, nil
}

func (m Model) handleUpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.viewMode == ViewModeLogs {
		var cmd tea.Cmd

		m.logsViewport, cmd = m.logsViewport.Update(msg)

		return m, cmd
	}

	if m.selected > 0 {
		m.selected--
		m.servicesViewport.YOffset = m.calculateScrollOffset()
	}

	return m, nil
}

func (m Model) handleDownKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.viewMode == ViewModeLogs {
		var cmd tea.Cmd

		m.logsViewport, cmd = m.logsViewport.Update(msg)

		return m, cmd
	}

	total := m.getTotalServices()
	if m.selected < total-1 {
		m.selected++
		m.servicesViewport.YOffset = m.calculateScrollOffset()
	}

	return m, nil
}

func (m Model) handleStopKey() (tea.Model, tea.Cmd) {
	if m.viewMode != ViewModeServices {
		return m, nil
	}

	service := m.getSelectedService()
	if service == nil || service.FSM == nil {
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

func (m Model) handleRestartKey() (tea.Model, tea.Cmd) {
	if m.viewMode != ViewModeServices {
		return m, nil
	}

	service := m.getSelectedService()
	if service == nil || service.FSM == nil {
		return m, nil
	}

	m.controller.Restart(m.ctx, service)

	return m, m.loader.Model.Tick
}

func (m Model) handleToggleLogSub() (tea.Model, tea.Cmd) {
	if m.viewMode != ViewModeServices {
		return m, nil
	}

	service := m.getSelectedService()
	if service != nil {
		service.LogEnabled = !service.LogEnabled

		m.updateLogsViewport()
	}

	return m, nil
}

func (m Model) handleEvent(event runtime.Event) (tea.Model, tea.Cmd) {
	switch event.Type {
	case runtime.EventProfileResolved:
		m = m.handleProfileResolved(event)
	case runtime.EventPhaseChanged:
		return m.handlePhaseChanged(event)
	case runtime.EventTierStarting:
		m = m.handleTierStarting(event)
	case runtime.EventTierReady:
		m = m.handleTierReady(event)
	case runtime.EventServiceStarting:
		m = m.handleServiceStarting(event)
	case runtime.EventServiceReady:
		m = m.handleServiceReady(event)
	case runtime.EventServiceFailed:
		m = m.handleServiceFailed(event)
	case runtime.EventServiceStopped:
		m = m.handleServiceStopped(event)
	case runtime.EventLogLine:
		m = m.handleLogLine(event)
	case runtime.EventSignalCaught:
		m.quitting = true
		m.loader.Start("_shutdown", "Shutting down all services…")

		return m, tea.Quit
	}

	return m, waitForEventCmd(m.eventChan)
}

func (m Model) handleProfileResolved(event runtime.Event) Model {
	data, ok := event.Data.(runtime.ProfileResolvedData)
	if !ok {
		m.log.Error().Msg("TUI: Failed to cast ProfileResolvedData")
		return m
	}

	m.log.Debug().Msgf("TUI: ProfileResolved - profile=%s, tiers=%d", data.Profile, len(data.Tiers))

	m.tiers = make([]TierView, len(data.Tiers))
	for i, tier := range data.Tiers {
		m.log.Debug().Msgf("TUI: Adding tier %s with %d services: %v", tier.Name, len(tier.Services), tier.Services)

		m.tiers[i] = TierView{Name: tier.Name, Services: tier.Services, Ready: false}
		for _, serviceName := range tier.Services {
			service := &ServiceState{Name: serviceName, Tier: tier.Name, Status: StatusStarting, LogEnabled: true}
			service.FSM = newServiceFSM(service, m.loader)
			m.services[serviceName] = service
		}
	}

	m.log.Debug().Msgf("TUI: After ProfileResolved - tiers=%d, services=%d", len(m.tiers), len(m.services))

	return m
}

func (m Model) handlePhaseChanged(event runtime.Event) (Model, tea.Cmd) {
	data, ok := event.Data.(runtime.PhaseChangedData)
	if !ok {
		return m, waitForEventCmd(m.eventChan)
	}

	m.phase = data.Phase
	if m.phase == runtime.PhaseStopped {
		m.loader.StopAll()
		return m, tea.Quit
	}

	return m, waitForEventCmd(m.eventChan)
}

func (m Model) handleTierStarting(event runtime.Event) Model {
	data, ok := event.Data.(runtime.TierStartingData)
	if !ok {
		return m
	}

	for i, tier := range m.tiers {
		if tier.Name == data.Name {
			m.tiers[i].Ready = false
			break
		}
	}

	return m
}

func (m Model) handleTierReady(event runtime.Event) Model {
	data, ok := event.Data.(runtime.TierReadyData)
	if !ok {
		return m
	}

	for i, tier := range m.tiers {
		if tier.Name == data.Name {
			m.tiers[i].Ready = true
			break
		}
	}

	return m
}

func (m Model) handleServiceStarting(event runtime.Event) Model {
	data, ok := event.Data.(runtime.ServiceStartingData)
	if !ok {
		return m
	}

	if service, exists := m.services[data.Service]; exists {
		service.Monitor.StartTime = event.Timestamp
		service.Tier = data.Tier
		m.controller.HandleStarting(m.ctx, service, data.PID)
	}

	return m
}

func (m Model) handleServiceReady(event runtime.Event) Model {
	data, ok := event.Data.(runtime.ServiceReadyData)
	if !ok {
		return m
	}

	if service, exists := m.services[data.Service]; exists {
		service.Monitor.ReadyTime = event.Timestamp

		m.loader.Stop(data.Service)
		m.controller.HandleReady(m.ctx, service)
	}

	return m
}

func (m Model) handleServiceFailed(event runtime.Event) Model {
	data, ok := event.Data.(runtime.ServiceFailedData)
	if !ok {
		return m
	}

	if service, exists := m.services[data.Service]; exists {
		service.Error = data.Error
		m.loader.Stop(data.Service)
		m.controller.HandleFailed(m.ctx, service)
	}

	return m
}

func (m Model) handleServiceStopped(event runtime.Event) Model {
	data, ok := event.Data.(runtime.ServiceStoppedData)
	if !ok {
		return m
	}

	if service, exists := m.services[data.Service]; exists {
		wasRestarting := m.controller.HandleStopped(m.ctx, service)
		if !wasRestarting {
			m.loader.Stop(data.Service)
		}
	}

	return m
}

func (m Model) handleLogLine(event runtime.Event) Model {
	data, ok := event.Data.(runtime.LogLineData)
	if !ok {
		return m
	}

	entry := LogEntry{
		Timestamp: event.Timestamp,
		Service:   data.Service,
		Tier:      data.Tier,
		Stream:    data.Stream,
		Message:   data.Message,
	}

	m.logs = append(m.logs, entry)

	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}

	m.updateLogsViewport()

	return m
}

func (m *Model) updateLogsViewport() {
	var logLines string

	viewportWidth := m.logsViewport.Width
	if viewportWidth <= 0 {
		viewportWidth = 80
	}

	for _, entry := range m.logs {
		serviceState, exists := m.services[entry.Service]
		if !exists || !serviceState.LogEnabled {
			continue
		}

		serviceName := entry.Service
		if len(serviceName) > maxLogServiceNameLength {
			serviceName = serviceName[:maxLogServiceNameLength] + "…"
		}

		service := serviceNameStyle.Render(serviceName)
		divider := timestampStyle.Render("·")

		prefix := fmt.Sprintf("%s %s ", service, divider)
		prefixLen := len(serviceName) + logPrefixSpacing

		message := strings.TrimRight(entry.Message, "\n\r")

		messageWidth := viewportWidth - prefixLen
		if messageWidth < minLogMessageWidth {
			messageWidth = minLogMessageWidth
		}

		wrappedMessage := logMessageStyle.Width(messageWidth).Render(message)

		lines := strings.Split(wrappedMessage, "\n")
		for i, line := range lines {
			linePrefix := prefix
			if i > 0 {
				linePrefix = "└ "
			}

			logLines += fmt.Sprintf("%s%s\n", linePrefix, line)
		}
	}

	m.logsViewport.SetContent(logLines)

	if m.autoscroll {
		m.logsViewport.GotoBottom()
	}
}

func waitForEventCmd(eventChan <-chan runtime.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-eventChan
		if !ok {
			return nil
		}

		return eventMsg(event)
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
