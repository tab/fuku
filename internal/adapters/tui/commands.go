package tui

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"fuku/internal/adapters/terminal"
	"fuku/internal/app/services"
	"fuku/internal/model"
)

// Control admits the view's service actions
type Control interface {
	Start(id string) (services.Admission, error)
	Stop(id string) (services.Admission, error)
	Restart(id string) (services.Admission, error)
	StopAll() error
}

// admissionMsg carries the core's answer to one service action of the view
type admissionMsg struct {
	name      string
	verb      string
	seenAt    time.Time // the service's LifecycleAt when the key was pressed
	admission services.Admission
	err       error
}

// stopAllMsg carries the core's answer to the stop of every service
type stopAllMsg struct {
	err error
}

// handleQuitKey asks the core to stop every service off the registry's read lock
func (m Model) handleQuitKey() (Model, tea.Cmd) {
	control := m.control

	return m, func() tea.Msg {
		return stopAllMsg{err: control.StopAll()}
	}
}

// handleStopKey toggles the selected service between running and stopped
func (m Model) handleStopKey() (Model, tea.Cmd) {
	service := m.getSelectedService()
	if service == nil {
		return m, nil
	}

	switch service.Status {
	case model.StatusStopped, model.StatusFailed:
		return m, admitCmd(service.ID, service.Name, service.LifecycleAt, m.control.Start, "starting")
	case model.StatusRunning:
		return m, admitCmd(service.ID, service.Name, service.LifecycleAt, m.control.Stop, "stopping")
	default:
		return m, nil
	}
}

// handleRestartKey restarts the selected service
func (m Model) handleRestartKey() (Model, tea.Cmd) {
	service := m.getSelectedService()
	if service == nil {
		return m, nil
	}

	return m, admitCmd(service.ID, service.Name, service.LifecycleAt, m.control.Restart, "restarting")
}

// handleRestartFailedKey restarts all services in the failed state
func (m Model) handleRestartFailedKey() (Model, tea.Cmd) {
	var cmds []tea.Cmd

	for _, id := range m.state.serviceIDs {
		svc := m.snapshot.Services[id]
		if svc.Status != model.StatusFailed {
			continue
		}

		cmds = append(cmds, admitCmd(svc.ID, svc.Name, svc.LifecycleAt, m.control.Restart, "restarting"))
	}

	return m, tea.Batch(cmds...)
}

// admitCmd runs one action through the core off the registry's read lock and answers with its admission
func admitCmd(id, name string, seenAt time.Time, action func(id string) (services.Admission, error), verb string) tea.Cmd {
	return func() tea.Msg {
		admission, err := action(id)

		return admissionMsg{name: name, verb: verb, seenAt: seenAt, admission: admission, err: err}
	}
}

// handleAdmission starts the service loader for an admitted action whose events have not arrived and logs a rejection
func (m Model) handleAdmission(msg admissionMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.log.Debug(fmt.Sprintf("TUI: %s of '%s' rejected", msg.verb, msg.name), "error", msg.err)

		return m, nil
	}

	service, known := m.snapshot.Services[msg.admission.Service.ID]
	if known && !service.LifecycleAt.Equal(msg.seenAt) {
		return m, nil
	}

	m.loader.Start(msg.admission.Service.ID, fmt.Sprintf("%s %s…", msg.verb, msg.name))

	return m, nil
}

// handleStopAll shows the shutdown loader when the core accepted the stop of every service and logs a rejection
func (m Model) handleStopAll(msg stopAllMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.log.Warn("TUI: Stop all rejected", "error", msg.err)

		return m, nil
	}

	m.state.shuttingDown = true
	m.loader.Start(loaderKeyShutdown, "shutting down all services…")

	return m, nil
}

// appStatsMsg contains sampled CPU and memory for the fuku process
type appStatsMsg struct {
	cpu float64
	mem float64
}

// sampleAppStatsCmd returns a command that samples fuku process stats off the UI thread
func (m Model) sampleAppStatsCmd() tea.Cmd {
	ctx := m.ctx
	mon := m.monitor

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, terminal.UIStatsCallTimeout)
		defer cancel()

		stats, err := mon.GetStats(ctx, os.Getpid())
		if err != nil {
			return appStatsMsg{}
		}

		return appStatsMsg{cpu: stats.CPU, mem: stats.MEM}
	}
}
