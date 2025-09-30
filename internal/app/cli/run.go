package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"fuku/internal/app/logs"
	"fuku/internal/app/procstats"
	"fuku/internal/app/readiness"
	"fuku/internal/app/runner"
	"fuku/internal/app/state"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// uiState manages UI viewports and selection
type uiState struct {
	serviceViewport viewport.Model
	logViewport     viewport.Model
	selectedIdx     int
	showLogs        bool
	width           int
	height          int
	ready           bool
}

// runModel is the main TUI model for the run command
type runModel struct {
	ctx     context.Context
	cancel  context.CancelFunc
	cfg     *config.Config
	log     logger.Logger
	profile string

	stateMgr       state.Manager
	logMgr         logs.Manager
	ui             *uiState
	spinner        spinner.Model
	controlChan    chan runner.ServiceControlRequest
	shutdownDone   chan struct{}
	isShuttingDown bool
	err            error
}

const (
	headerHeight          = 2
	helpHeight            = 3
	logsLabelHeight       = 1
	viewportBorderPadding = 4
	minServiceLines       = 5
)

var (
	viewportStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			PaddingLeft(1).
			PaddingRight(1)

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("240")).
			Foreground(lipgloss.Color("15"))

	dimmedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

func newRunModel(ctx context.Context, cfg *config.Config, log logger.Logger, profile string) runModel {
	ctx, cancel := context.WithCancel(ctx)

	serviceVp := viewport.New(80, 20)
	serviceVp.Style = viewportStyle

	logVp := viewport.New(80, 10)
	logVp.YPosition = 0

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return runModel{
		ctx:      ctx,
		cancel:   cancel,
		cfg:      cfg,
		log:      log,
		profile:  profile,
		stateMgr: state.NewManager(),
		logMgr:   logs.NewManager(),
		ui: &uiState{
			serviceViewport: serviceVp,
			logViewport:     logVp,
			selectedIdx:     0,
			showLogs:        false,
		},
		spinner:        s,
		controlChan:    make(chan runner.ServiceControlRequest, 10),
		shutdownDone:   make(chan struct{}),
		isShuttingDown: false,
	}
}

func (m runModel) Init() tea.Cmd {
	return tea.Batch(
		m.startRunner(),
		tea.WindowSize(),
		m.tickResourceStats(),
		m.spinner.Tick,
	)
}

type tickMsg time.Time

func (m runModel) tickResourceStats() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *runModel) updateResourceStats() {
	for _, svc := range m.stateMgr.GetServices() {
		if svc.PID > 0 && (svc.State == state.Initializing || svc.State == state.Ready || svc.State == state.Running || svc.State == state.Stopping) {
			stats := procstats.GetStats(svc.PID)
			svc.CPUUsage = stats.CPUPercent
			svc.MemUsage = stats.MemoryBytes
			m.stateMgr.AddService(svc)
		}
	}
}

func (m runModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		return m.handleWindowResize(msg)
	case runner.Event:
		updatedModel, eventCmd := m.handleRunnerEvent(msg)
		m = updatedModel.(runModel)
		if eventCmd != nil {
			cmds = append(cmds, eventCmd)
		}
	case tickMsg:
		m.updateResourceStats()
		m.updateServiceViewport()
		cmds = append(cmds, m.tickResourceStats())
	case shutdownCompleteMsg:
		return m, tea.Quit
	case errMsg:
		m.err = msg.err
		return m, tea.Quit
	}

	phase := m.stateMgr.GetPhase()
	if phase == runner.PhaseDiscovery || phase == runner.PhaseExecution || phase == runner.PhaseShutdown {
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		if spinnerCmd != nil {
			cmds = append(cmds, spinnerCmd)
		}
	}

	var viewportCmd tea.Cmd
	if m.ui.showLogs {
		m.ui.logViewport, viewportCmd = m.ui.logViewport.Update(msg)
	} else {
		m.ui.serviceViewport, viewportCmd = m.ui.serviceViewport.Update(msg)
	}
	if viewportCmd != nil {
		cmds = append(cmds, viewportCmd)
	}

	return m, tea.Batch(cmds...)
}

type shutdownCompleteMsg struct{}

func (m runModel) waitForShutdown() tea.Cmd {
	return func() tea.Msg {
		<-m.shutdownDone
		return shutdownCompleteMsg{}
	}
}

func (m runModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if !m.isShuttingDown {
			m.isShuttingDown = true
			m.cancel()
			return m, m.waitForShutdown()
		}
		return m, nil
	case "l":
		m.ui.showLogs = !m.ui.showLogs
	case "up", "k":
		m.handleUpKey()
	case "down", "j":
		m.handleDownKey()
	case "pgup":
		m.handlePageUp()
	case "pgdown":
		m.handlePageDown()
	case "home":
		m.handleHome()
	case "end":
		m.handleEnd()
	case " ":
		m.handleSpaceKey()
	case "a":
		m.handleSelectAll()
	case "s":
		m.handleStopStart()
	case "r":
		m.handleRestart()
	}
	return m, nil
}

func (m *runModel) handleUpKey() {
	if m.ui.showLogs {
		m.ui.logViewport.ScrollUp(1)
	} else if m.ui.selectedIdx > 0 {
		m.ui.selectedIdx--
		m.updateServiceViewport()
	}
}

func (m *runModel) handleDownKey() {
	if m.ui.showLogs {
		m.ui.logViewport.ScrollDown(1)
	} else if m.ui.selectedIdx < len(m.stateMgr.GetServiceOrder())-1 {
		m.ui.selectedIdx++
		m.updateServiceViewport()
	}
}

func (m *runModel) handlePageUp() {
	if m.ui.showLogs {
		m.ui.logViewport.PageUp()
	} else {
		m.ui.serviceViewport.PageUp()
	}
}

func (m *runModel) handlePageDown() {
	if m.ui.showLogs {
		m.ui.logViewport.PageDown()
	} else {
		m.ui.serviceViewport.PageDown()
	}
}

func (m *runModel) handleHome() {
	if m.ui.showLogs {
		m.ui.logViewport.GotoTop()
	} else {
		m.ui.selectedIdx = 0
		m.updateServiceViewport()
	}
}

func (m *runModel) handleEnd() {
	if m.ui.showLogs {
		m.ui.logViewport.GotoBottom()
	} else if len(m.stateMgr.GetServiceOrder()) > 0 {
		m.ui.selectedIdx = len(m.stateMgr.GetServiceOrder()) - 1
		m.updateServiceViewport()
	}
}

func (m *runModel) handleSpaceKey() {
	if !m.ui.showLogs && m.ui.selectedIdx < len(m.stateMgr.GetServiceOrder()) {
		serviceName := m.stateMgr.GetServiceOrder()[m.ui.selectedIdx]
		m.logMgr.ToggleFilter(serviceName)
		m.updateServiceViewport()
		m.updateLogViewport()
	}
}

func (m *runModel) handleSelectAll() {
	m.logMgr.AddAllFilters(m.stateMgr.GetServiceOrder())
	m.updateServiceViewport()
	m.updateLogViewport()
}

func (m *runModel) handleStopStart() {
	if m.ui.showLogs || m.ui.selectedIdx >= len(m.stateMgr.GetServiceOrder()) {
		return
	}
	serviceName := m.stateMgr.GetServiceOrder()[m.ui.selectedIdx]
	svc, exists := m.stateMgr.GetService(serviceName)
	if !exists {
		return
	}
	switch svc.State {
	case state.Starting, state.Initializing, state.Ready, state.Running:
		svc.State = state.Stopping
		svc.PID = 0
		m.stateMgr.AddService(svc)
		m.updateServiceViewport()
		m.controlChan <- runner.ServiceControlRequest{ServiceName: serviceName, Action: runner.ControlStop}
	case state.Stopped, state.Failed:
		svc.State = state.Starting
		svc.PID = 0
		m.stateMgr.AddService(svc)
		m.updateServiceViewport()
		m.controlChan <- runner.ServiceControlRequest{ServiceName: serviceName, Action: runner.ControlStart}
	}
}

func (m *runModel) handleRestart() {
	if m.ui.showLogs || m.ui.selectedIdx >= len(m.stateMgr.GetServiceOrder()) {
		return
	}
	serviceName := m.stateMgr.GetServiceOrder()[m.ui.selectedIdx]
	svc, exists := m.stateMgr.GetService(serviceName)
	if exists && (svc.State == state.Initializing || svc.State == state.Ready || svc.State == state.Running) {
		svc.State = state.Restarting
		svc.PID = 0
		m.stateMgr.AddService(svc)
		m.updateServiceViewport()
		m.controlChan <- runner.ServiceControlRequest{ServiceName: serviceName, Action: runner.ControlRestart}
	}
}

func (m runModel) handleWindowResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.ui.width = msg.Width
	m.ui.height = msg.Height

	availableHeight := msg.Height - headerHeight - helpHeight - 2

	if !m.ui.ready {
		m.ui.serviceViewport = viewport.New(msg.Width-4, availableHeight)
		m.ui.serviceViewport.Style = viewportStyle

		m.ui.logViewport = viewport.New(msg.Width, availableHeight)
		m.ui.ready = true
	} else {
		m.ui.serviceViewport.Width = msg.Width - 4
		m.ui.serviceViewport.Height = availableHeight

		m.ui.logViewport.Width = msg.Width
		m.ui.logViewport.Height = availableHeight
	}

	m.updateServiceViewport()
	m.updateLogViewport()
	return m, nil
}

func (m runModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	var b strings.Builder

	total, running, starting, stopped, failed := m.getServiceCounts()
	phaseStr := m.getPhaseString()
	filterStr := m.getFilterString()

	var counters strings.Builder
	fmt.Fprintf(&counters, "%d/%d running", running, total)
	if starting > 0 {
		fmt.Fprintf(&counters, ", %d starting", starting)
	}
	if failed > 0 {
		fmt.Fprintf(&counters, ", %d failed", failed)
	}
	if stopped > 0 {
		fmt.Fprintf(&counters, ", %d stopped", stopped)
	}

	header := headlineLarge.Render(fmt.Sprintf("Fuku - %s | %s | %s | Filter: %s", m.profile, phaseStr, counters.String(), filterStr))
	b.WriteString(header)
	b.WriteString("\n\n")

	if m.ui.showLogs {
		b.WriteString(m.ui.logViewport.View())
	} else {
		b.WriteString(m.ui.serviceViewport.View())
	}
	b.WriteString("\n\n")

	viewName := "Services"
	helpText := "View: %s • l: toggle logs • space: filter • a: all • s: stop/start • r: restart • ↑/↓: navigate • q: quit"
	if m.ui.showLogs {
		viewName = "Logs"
		helpText = "View: %s • l: toggle logs • ↑/↓: navigate • q: quit"
	}

	help := labelLarge.Render(fmt.Sprintf(helpText, viewName))
	b.WriteString(help)

	return b.String()
}

func (m *runModel) updateServiceViewport() {
	content := m.renderServiceList()
	m.ui.serviceViewport.SetContent(content)

	yPosition := 0
	for i := 0; i < m.ui.selectedIdx && i < len(m.stateMgr.GetServiceOrder()); i++ {
		yPosition++
	}
	m.ui.serviceViewport.SetYOffset(yPosition)
}

func (m *runModel) updateLogViewport() {
	filteredLogs := m.logMgr.GetFilteredLogs()
	lines := make([]string, 0, len(filteredLogs))

	viewportWidth := m.ui.logViewport.Width
	if viewportWidth <= 0 {
		viewportWidth = 80
	}

	for _, entry := range filteredLogs {
		wrapped := wrapText(entry.Text, viewportWidth)
		lines = append(lines, wrapped...)
	}
	m.ui.logViewport.SetContent(strings.Join(lines, "\n"))
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		width = 80
	}

	visibleLen := getVisibleLength(text)
	if visibleLen <= width {
		return []string{text}
	}

	var lines []string
	for len(text) > 0 {
		breakPoint := findBreakPoint(text, width)
		if breakPoint >= len(text) {
			lines = append(lines, text)
			break
		}

		lines = append(lines, text[:breakPoint])
		text = text[breakPoint:]
	}

	return lines
}

func getVisibleLength(text string) int {
	length := 0
	inEscape := false

	for i := 0; i < len(text); i++ {
		switch {
		case text[i] == '\x1b':
			inEscape = true
		case inEscape && text[i] == 'm':
			inEscape = false
		case !inEscape:
			length++
		}
	}

	return length
}

func findBreakPoint(text string, width int) int {
	visibleCount := 0
	inEscape := false

	for i := 0; i < len(text); i++ {
		switch {
		case text[i] == '\x1b':
			inEscape = true
		case inEscape && text[i] == 'm':
			inEscape = false
		case !inEscape:
			visibleCount++
			if visibleCount >= width {
				return i + 1
			}
		}
	}

	return len(text)
}

func (m runModel) getServiceCounts() (total, running, starting, stopped, failed int) {
	return m.stateMgr.GetServiceCounts()
}

func (m runModel) getPhaseString() string {
	phase := m.stateMgr.GetPhase()
	switch phase {
	case runner.PhaseDiscovery:
		return fmt.Sprintf("%sDiscovery", m.spinner.View())
	case runner.PhaseExecution:
		return fmt.Sprintf("%sStarting services", m.spinner.View())
	case runner.PhaseRunning:
		return "✅ Running"
	case runner.PhaseShutdown:
		return fmt.Sprintf("%sStopping services", m.spinner.View())
	default:
		return "Unknown"
	}
}

func (m runModel) getFilterString() string {
	count := m.logMgr.GetFilterCount()
	if count == 0 {
		return "None"
	}
	if count == len(m.stateMgr.GetServiceOrder()) {
		return "All services"
	}
	if count == 1 {
		for name, filtered := range m.logMgr.GetFilteredNames() {
			if filtered {
				return name
			}
		}
	}
	return fmt.Sprintf("%d services", count)
}

func (m runModel) renderServiceList() string {
	serviceOrder := m.stateMgr.GetServiceOrder()
	lines := make([]string, 0, len(serviceOrder))

	for i, serviceName := range serviceOrder {
		svc, exists := m.stateMgr.GetService(serviceName)
		if !exists {
			continue
		}

		checkbox := "[ ]"
		if m.logMgr.IsFiltered(svc.Name) {
			checkbox = "[x]"
		}

		statusStr := svc.State.String()
		pidStr := "-      "
		if svc.PID > 0 {
			pidStr = fmt.Sprintf("%-7d", svc.PID)
		}

		cpuStr := "  -  "
		memStr := "   -   "
		uptimeStr := "   -   "

		if svc.State == state.Initializing || svc.State == state.Ready || svc.State == state.Running || svc.State == state.Stopping {
			if svc.CPUUsage >= 0 {
				cpuStr = fmt.Sprintf("%4.1f%%", svc.CPUUsage)
			}
			if svc.MemUsage > 0 {
				memStr = procstats.FormatMemory(svc.MemUsage)
			}
			if !svc.StartTime.IsZero() {
				uptimeStr = procstats.FormatUptime(time.Since(svc.StartTime))
			}
		}

		line := fmt.Sprintf(" %s %-50s %s  %s  %s  %s  %s",
			checkbox, svc.Name, statusStr, pidStr, cpuStr, memStr, uptimeStr)

		if !m.ui.showLogs && i == m.ui.selectedIdx {
			line = selectedStyle.Render(line)
		} else if !m.logMgr.IsFiltered(svc.Name) {
			line = dimmedStyle.Render(line)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m runModel) handleRunnerEvent(event runner.Event) (tea.Model, tea.Cmd) {
	switch event.Type {
	case runner.EventPhaseStart:
		if data, ok := event.Data.(runner.PhaseStart); ok {
			m.stateMgr.SetPhase(data.Phase)
			if data.Phase == runner.PhaseShutdown {
				for _, svcName := range m.stateMgr.GetServiceOrder() {
					if svc, exists := m.stateMgr.GetService(svcName); exists && svc.State == state.Running {
						svc.State = state.Stopping
						m.stateMgr.AddService(svc)
					}
				}
				return m, m.spinner.Tick
			}
			m.updateServiceViewport()
		}

	case runner.EventPhaseDone:
		if data, ok := event.Data.(runner.PhaseDone); ok {
			if data.Phase == runner.PhaseDiscovery {
				m.stateMgr.SetTotalServices(data.ServiceCount)
				for _, svcName := range data.ServiceNames {
					m.stateMgr.AddService(&state.ServiceStatus{
						Name:  svcName,
						State: state.Starting,
					})
				}
				m.logMgr.AddAllFilters(data.ServiceNames)
				m.updateServiceViewport()
				m.updateLogViewport()
			}
			if data.Phase == runner.PhaseShutdown {
				close(m.shutdownDone)
			}
		}

	case runner.EventServiceStart:
		if data, ok := event.Data.(runner.ServiceStart); ok {
			svc := &state.ServiceStatus{
				Name:      data.Name,
				State:     state.Initializing,
				PID:       data.PID,
				StartTime: data.StartTime,
			}
			m.stateMgr.AddService(svc)
			m.updateServiceViewport()
		}

	case runner.EventServiceReady:
		if data, ok := event.Data.(runner.ServiceReady); ok {
			if svc, exists := m.stateMgr.GetService(data.Name); exists {
				svc.State = state.Ready
				m.stateMgr.AddService(svc)
				m.updateServiceViewport()
			}
		}

	case runner.EventServiceLog:
		if data, ok := event.Data.(runner.ServiceLog); ok {
			timestamp := data.Time.Format("2006-01-02T15:04:05.000000")
			coloredTimestamp := logTimestampStyle.Render(fmt.Sprintf("[%s]", timestamp))
			coloredServiceName := logServiceNameStyle.Render(fmt.Sprintf("[%s]", data.Name))
			logLine := fmt.Sprintf("%s %s %s", coloredTimestamp, coloredServiceName, data.Line)
			m.logMgr.AddLog(data.Name, logLine)
			m.updateLogViewport()
			m.ui.logViewport.GotoBottom()
		}

	case runner.EventServiceStop:
		if data, ok := event.Data.(runner.ServiceStop); ok {
			if svc, exists := m.stateMgr.GetService(data.Name); exists {
				if data.ExitCode == 0 || data.GracefulStop {
					svc.State = state.Stopped
				} else {
					svc.State = state.Failed
				}
				svc.PID = 0
				svc.ExitCode = data.ExitCode
				m.stateMgr.AddService(svc)
				m.updateServiceViewport()
			}
		}

	case runner.EventServiceFail:
		if data, ok := event.Data.(runner.ServiceFail); ok {
			if svc, exists := m.stateMgr.GetService(data.Name); exists {
				svc.State = state.Failed
				svc.PID = 0
				svc.Err = data.Error
				m.stateMgr.AddService(svc)
				m.updateServiceViewport()
			}
		}

	case runner.EventError:
		if data, ok := event.Data.(runner.ErrorData); ok {
			m.err = data.Error
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m runModel) startRunner() tea.Cmd {
	eventChan := make(chan runner.Event, 100)

	callback := func(e runner.Event) {
		eventChan <- e
	}

	factory := readiness.NewFactory()
	r := runner.NewRunner(m.cfg, factory, m.log, callback)

	go func() {
		for {
			select {
			case ctrl := <-m.controlChan:
				r.GetControlChannel() <- ctrl
			case <-m.ctx.Done():
				return
			}
		}
	}()

	go func() {
		if err := r.Run(m.ctx, m.profile); err != nil {
			m.log.Error().Err(err).Msg("Runner failed")
			select {
			case eventChan <- runner.Event{
				Type:      runner.EventError,
				Timestamp: time.Now(),
				Data:      runner.ErrorData{Error: err, Time: time.Now()},
			}:
			case <-m.ctx.Done():
			}
		}
		close(eventChan)
	}()

	return waitForEvent(eventChan)
}

type errMsg struct {
	err error
}

func waitForEvent(eventChan chan runner.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-eventChan
		if !ok {
			return tea.Quit()
		}
		return tea.Batch(
			func() tea.Msg { return event },
			waitForEvent(eventChan),
		)()
	}
}
