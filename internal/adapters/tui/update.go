package tui

import (
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"fuku/internal/adapters/terminal"
	"fuku/internal/contracts"
	"fuku/internal/model"
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

// EventMsg carries a bus message into the program
type EventMsg contracts.Message

// tickMsg signals a UI tick for animations
type tickMsg time.Time

// Update handles one message under the registry's read lock, so every fact it reads comes from one snapshot
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		next Model
		cmd  tea.Cmd
	)

	m.registry.Read(func(snapshot *model.Snapshot) {
		m.snapshot = snapshot
		next, cmd = m.update(msg)
		next.snapshot = nil
	})

	return next, cmd
}

// update handles one message with the read model bound
func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.ui.width = msg.Width
		m.ui.height = msg.Height
		m.ui.help.SetWidth(msg.Width)

		m.ensureAsideShowable()

		m = m.recomputeLayout()
		m.recomputeViewport()

		if !m.state.ready {
			m.state.ready = true
		}

		m.updateServicesContent()
		m.updateAsideContent()

		return m, nil

	case tea.BackgroundColorMsg:
		isDark := msg.IsDark()

		m.log.Debug("TUI: Background color detected", "isDark", isDark, "color", msg.String())

		appearance := terminal.AppearanceLight
		if isDark {
			appearance = terminal.AppearanceDark
		}

		m.theme = terminal.NewTheme(appearance)
		m.ui.help.Styles = help.DefaultStyles(isDark)
		m.updateServicesContent()
		m.updateAsideContent()

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

		var cmds []tea.Cmd

		if m.state.now.IsZero() || m.ui.tickCounter%terminal.UITicksPerSecond == 0 {
			m.state.now = time.Now()
			cmds = append(cmds, m.applySnapshot())
			m.sampleTimelines()
			cmds = append(cmds, m.sampleAppStatsCmd())
		}

		m.updateBlinkAnimations()
		m.updateServicesContent()
		m.updateAsideContent()

		return m, tea.Batch(append(cmds, tickCmd())...)

	case appStatsMsg:
		m.state.appCPU = msg.cpu
		m.state.appMEM = msg.mem

		return m, nil

	case admissionMsg:
		return m.handleAdmission(msg)

	case stopAllMsg:
		return m.handleStopAll(msg)

	case EventMsg:
		return m.handleMessage(contracts.Message(msg))
	}

	return m, nil
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if key.Matches(msg, m.ui.servicesKeys.ForceQuit) {
		m.log.Warn("TUI: Force quit requested, exiting immediately")
		m.loader.StopAll()

		return m, tea.Quit
	}

	if m.state.shuttingDown {
		return m, nil
	}

	if m.state.filterActive {
		return m.handleFilterInput(msg)
	}

	if m.state.asideOpen {
		switch {
		case key.Matches(msg, m.ui.servicesKeys.AsideClose),
			key.Matches(msg, m.ui.servicesKeys.OpenAside):
			return m.setAsideOpen(false), nil
		case key.Matches(msg, m.ui.servicesKeys.AsideTabNext):
			return m.handleAsideTabNext()
		case key.Matches(msg, m.ui.servicesKeys.AsideTabPrev):
			return m.handleAsideTabPrev()
		case key.Matches(msg, m.ui.servicesKeys.FocusToggle):
			return m.handleFocusToggle()
		}
	}

	switch {
	case key.Matches(msg, m.ui.servicesKeys.Quit):
		return m.handleQuitKey()

	case key.Matches(msg, m.ui.servicesKeys.Up):
		return m.handleMoveKey(msg, -1)

	case key.Matches(msg, m.ui.servicesKeys.Down):
		return m.handleMoveKey(msg, 1)

	case key.Matches(msg, m.ui.servicesKeys.Stop):
		return m.handleStopKey()

	case key.Matches(msg, m.ui.servicesKeys.RestartFailed):
		return m.handleRestartFailedKey()

	case key.Matches(msg, m.ui.servicesKeys.Restart):
		return m.handleRestartKey()

	case key.Matches(msg, m.ui.servicesKeys.Filter):
		return m.handleFilterKey()

	case key.Matches(msg, m.ui.servicesKeys.OpenAside):
		return m.handleOpenAsideKey()

	case key.Matches(msg, m.ui.servicesKeys.ClearFilter):
		if m.state.filterQuery != "" {
			m.clearFilter()

			return m, nil
		}

	case key.Matches(msg, m.ui.servicesKeys.ToggleTips):
		m.ui.showTips = !m.ui.showTips
		return m, nil
	}

	if m.state.asideFocused {
		switch msg.String() {
		case "home":
			m.ui.asideViewport.GotoTop()

			return m, nil
		case "end":
			m.ui.asideViewport.GotoBottom()

			return m, nil
		case "pgup", "pgdown":
			var cmd tea.Cmd

			m.ui.asideViewport, cmd = m.ui.asideViewport.Update(msg)

			return m, cmd
		}

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

// handleOpenAsideKey opens the aside panel for the currently selected service
func (m Model) handleOpenAsideKey() (Model, tea.Cmd) {
	if m.state.asideOpen || m.getSelectedService() == nil || !m.canShowAside() {
		return m, nil
	}

	return m.setAsideOpen(true), nil
}

// handleAsideTabNext switches to the next aside tab
func (m Model) handleAsideTabNext() (Model, tea.Cmd) {
	m.state.asideTab = nextAsideTab(m.state.asideTab)
	m.updateAsideContent()
	m.ui.asideViewport.SetYOffset(0)

	return m, nil
}

// handleAsideTabPrev switches to the previous aside tab
func (m Model) handleAsideTabPrev() (Model, tea.Cmd) {
	m.state.asideTab = prevAsideTab(m.state.asideTab)
	m.updateAsideContent()
	m.ui.asideViewport.SetYOffset(0)

	return m, nil
}

// handleFocusToggle moves keyboard focus between the services and aside panels
func (m Model) handleFocusToggle() (Model, tea.Cmd) {
	m.state.asideFocused = !m.state.asideFocused

	return m, nil
}

// setAsideOpen toggles the aside open state and refreshes layout, viewport, focus, and scroll offset
func (m Model) setAsideOpen(open bool) Model {
	m.state.asideOpen = open
	m.state.asideFocused = open
	m = m.recomputeLayout()
	m.recomputeViewport()
	m.refreshSelection()

	return m
}

// handleMoveKey scrolls the aside viewport when aside-focused, otherwise moves the selection by delta
func (m Model) handleMoveKey(msg tea.KeyPressMsg, delta int) (Model, tea.Cmd) {
	if m.state.asideFocused {
		var cmd tea.Cmd

		m.ui.asideViewport, cmd = m.ui.asideViewport.Update(msg)

		return m, cmd
	}

	next := m.state.selected + delta
	if next < 0 || next >= len(m.activeServiceIDs()) {
		return m, nil
	}

	m.state.selected = next
	m.refreshSelection()

	return m, nil
}

// refreshSelection rebuilds both panels for the selection, scrolls it into view and resets the aside scroll
func (m *Model) refreshSelection() {
	m.updateServicesContent()
	m.updateAsideContent()
	m.ui.servicesViewport.SetYOffset(m.calculateScrollOffset())
	m.ui.asideViewport.SetYOffset(0)
}

// tickCmd returns a command that sends a tick after the interval
func tickCmd() tea.Cmd {
	return tea.Tick(terminal.UITickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
