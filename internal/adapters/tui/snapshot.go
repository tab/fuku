package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"fuku/internal/adapters/terminal"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

// handleMessage dispatches a bus message: snapshots carry the read model, the rest are raw events
func (m Model) handleMessage(msg contracts.Message) (Model, tea.Cmd) {
	//nolint:exhaustive // only handling events relevant to UI
	switch msg.Type {
	case contracts.EventSnapshotChanged:
		cmd := m.applySnapshot()

		return m, cmd
	case contracts.EventPreflightStarted:
		m = m.handlePreflightStarted()
	case contracts.EventPreflightKilled:
		m = m.handlePreflightKill(msg)
	case contracts.EventPreflightComplete:
		m = m.handlePreflightComplete()
	case contracts.EventSignalReceived:
		m = m.handleSignal()
	case contracts.EventUpdateAvailable:
		m = m.handleUpdateAvailable(msg)
	}

	return m, nil
}

// applySnapshot runs the effects of every change since the view last looked and quits once the run stopped
func (m *Model) applySnapshot() tea.Cmd {
	if m.snapshot.Resolved && !m.state.resolved {
		m.resolve()
	}

	for _, id := range m.state.serviceIDs {
		m.applyService(m.state.views[id], m.snapshot.Services[id])
	}

	if m.snapshot.Phase == model.PhaseStopped {
		m.loader.StopAll()

		return tea.Quit
	}

	return nil
}

// resolve builds the view state from the resolved profile in tier order and leaves the loader alone
func (m *Model) resolve() {
	m.log.Debug(fmt.Sprintf("TUI: Profile resolved - profile=%s, tiers=%d", m.snapshot.Profile, len(m.snapshot.Tiers)))

	m.state.resolved = true
	m.state.views = make(map[string]*serviceView, len(m.snapshot.Services))
	m.state.restarting = make(map[string]bool)
	m.state.serviceIDs = nil
	m.state.selected = 0
	m.state.filterQuery = ""
	m.state.filterActive = false
	m.state.filteredIDs = nil
	m.state.preFilterSelectedID = ""
	m.state.lastFilteredSelectedID = ""

	for _, tier := range m.snapshot.Tiers {
		for _, svc := range tier.Services {
			m.state.serviceIDs = append(m.state.serviceIDs, svc.ID)
			m.state.views[svc.ID] = &serviceView{
				Status:   model.StatusPending,
				Blink:    terminal.NewBlink(),
				Timeline: newTimeline(terminal.TimelineDefaultSlots),
			}
		}
	}

	m.ensureAsideShowable()
	*m = m.recomputeLayout()
}

// applyService records the status and attempt the view saw and runs the effects of a status change or a new attempt
func (m *Model) applyService(view *serviceView, service *model.Service) {
	previous := view.Status
	newProcess := service.AttemptedAt != view.AttemptedAt

	view.Status = service.Status
	view.AttemptedAt = service.AttemptedAt

	if newProcess {
		view.StartupSampled = 0
	}

	if previous == service.Status && !newProcess {
		return
	}

	//nolint:exhaustive // pending has no effect
	switch service.Status {
	case model.StatusStarting:
		view.StartupActive = true

		delete(m.state.restarting, service.ID)

		if !m.loader.Has(service.ID) {
			m.loader.Start(service.ID, fmt.Sprintf("starting %s…", service.Name))
		}
	case model.StatusRunning, model.StatusFailed:
		m.settle(view, service, newProcess)
		delete(m.state.restarting, service.ID)
		m.loader.Stop(service.ID)
	case model.StatusStopped:
		m.settle(view, service, newProcess)

		if !m.state.restarting[service.ID] {
			m.loader.Stop(service.ID)
		}
	case model.StatusStopping:
		if !m.loader.Has(service.ID) {
			m.loader.Start(service.ID, fmt.Sprintf("stopping %s…", service.Name))
		}
	case model.StatusRestarting:
		m.state.restarting[service.ID] = true
		m.loader.Start(service.ID, fmt.Sprintf("restarting %s…", service.Name))
	}
}

// settle ends a startup attempt and backfills the amber slots the timeline missed since the attempt started
func (m *Model) settle(view *serviceView, service *model.Service, newProcess bool) {
	if view.StartupActive || newProcess {
		backfillStartupHistory(view, service.AttemptedAt, service.LifecycleAt)
	}

	view.StartupActive = false
}

// handleUpdateAvailable stores the latest version when a newer release is announced
func (m Model) handleUpdateAvailable(msg contracts.Message) Model {
	data, ok := msg.Data.(contracts.UpdateAvailable)
	if !ok {
		return m
	}

	m.state.availableVersion = data.Version

	return m
}

// handlePreflightStarted updates loader when preflight scan begins
func (m Model) handlePreflightStarted() Model {
	m.loader.Start(loaderKeyPreflight, "preflight: scanning processes…")

	return m
}

// handlePreflightKill updates loader with the service being killed
func (m Model) handlePreflightKill(msg contracts.Message) Model {
	data, ok := msg.Data.(contracts.PreflightKilled)
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
