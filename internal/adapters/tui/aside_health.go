package tui

import (
	"strconv"
	"strings"
	"time"

	"fuku/internal/model"
)

// asideHealthTab renders the health tab content with probe, process, retry, and status cards stacked vertically
func (m Model) asideHealthTab(service *model.Service, innerWidth int) string {
	cards := make([]string, 0, 4)

	if probe := m.asideProbeCard(service, innerWidth); probe != "" {
		cards = append(cards, probe)
	}

	if proc := m.asideProcessCard(service, innerWidth); proc != "" {
		cards = append(cards, proc)
	}

	if retry := m.asideRetryCard(innerWidth); retry != "" {
		cards = append(cards, retry)
	}

	if status := m.asideStatusCard(service, innerWidth); status != "" {
		cards = append(cards, status)
	}

	return strings.Join(cards, "\n\n")
}

// asideProbeCard renders the readiness probe configuration as a meta-styled card
func (m Model) asideProbeCard(service *model.Service, innerWidth int) string {
	if service.Readiness == nil || service.Readiness.Type == "" {
		return ""
	}

	r := service.Readiness
	rows := m.readinessRows(r)

	if r.Interval > 0 {
		rows = append(rows, cardRow{label: "interval", value: r.Interval.String()})
	}

	if r.Timeout > 0 {
		rows = append(rows, cardRow{label: "timeout", value: r.Timeout.String()})
	}

	return m.asideSection("probe", rows, innerWidth)
}

// asideProcessCard renders the process id and uptime, with "—" while unknown so the card survives a restart
func (m Model) asideProcessCard(service *model.Service, innerWidth int) string {
	pidValue := asidePlaceholder
	pidStyle := m.theme.PanelMutedStyle

	if service.Process.PID != 0 {
		pidValue = strconv.Itoa(service.Process.PID)
		pidStyle = m.theme.StatusRunningStyle
	}

	uptimeValue := asidePlaceholder
	if uptime := m.getUptime(service); uptime != "" {
		uptimeValue = uptime
	}

	rows := []cardRow{
		{label: "pid", value: pidValue, style: pidStyle},
		{label: "uptime", value: uptimeValue},
	}

	return m.asideSection("process", rows, innerWidth)
}

// asideRetryCard renders the global retry policy
func (m Model) asideRetryCard(innerWidth int) string {
	attempts := m.retryAttempts
	backoff := m.retryBackoff

	if attempts == 0 && backoff == 0 {
		return ""
	}

	rows := make([]cardRow, 0, 2)

	if attempts > 0 {
		rows = append(rows, cardRow{label: "attempts", value: strconv.Itoa(attempts), style: m.theme.PhaseStartingStyle})
	}

	if backoff > 0 {
		rows = append(rows, cardRow{label: "backoff", value: backoff.String()})
	}

	return m.asideSection("retry", rows, innerWidth)
}

// asideStatusCard renders the current lifecycle state and how long it has held
func (m Model) asideStatusCard(service *model.Service, innerWidth int) string {
	rows := []cardRow{
		{label: "state", value: string(service.Status), style: m.statusStyle(service.Status)},
	}

	if !service.LifecycleAt.IsZero() && !m.state.now.IsZero() {
		rows = append(rows, cardRow{label: "duration", value: formatElapsed(m.state.now.Sub(service.LifecycleAt))})
	}

	return m.asideSection("status", rows, innerWidth)
}

// formatElapsed renders a duration as HH:MM:SS (or MM:SS when under an hour)
func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return pad(hours) + ":" + pad(minutes) + ":" + pad(seconds)
	}

	return pad(minutes) + ":" + pad(seconds)
}
