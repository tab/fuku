package services

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/looplab/fsm"

	"fuku/internal/app/runtime"
	"fuku/internal/config/logger"
)

// Status represents the status of a service
type Status string

const (
	StatusStarting Status = "Starting"
	StatusReady    Status = "Ready"
	StatusStopping Status = "Stopping"
	StatusFailed   Status = "Failed"
	StatusStopped  Status = "Stopped"
)

// ViewMode represents the current view mode
type ViewMode int

const (
	ViewModeServices ViewMode = iota
	ViewModeLogs
)

// TierView represents a tier in the UI
type TierView struct {
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
	Name       string
	Tier       string
	Status     Status
	Error      error
	FSM        *fsm.FSM
	Monitor    ServiceMonitor
	LogEnabled bool
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

// LogEntry represents a single log line
type LogEntry struct {
	Timestamp time.Time
	Service   string
	Tier      string
	Stream    string
	Message   string
}

// Model represents the Bubble Tea model for the services UI
type Model struct {
	ctx              context.Context
	profile          string
	phase            runtime.Phase
	tiers            []TierView
	services         map[string]*ServiceState
	selected         int
	width            int
	height           int
	keys             KeyMap
	help             help.Model
	event            runtime.EventBus
	command          runtime.CommandBus
	loader           *Loader
	quitting         bool
	viewMode         ViewMode
	logs             []LogEntry
	maxLogs          int
	servicesViewport viewport.Model
	logsViewport     viewport.Model
	autoscroll       bool
	ready            bool
	eventChan        <-chan runtime.Event
	log              logger.Logger
}

// NewModel creates a new services UI model
func NewModel(
	ctx context.Context,
	profile string,
	event runtime.EventBus,
	command runtime.CommandBus,
	log logger.Logger,
) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	eventChan := event.Subscribe(ctx)

	log.Debug().Msg("TUI: Created model and subscribed to events")

	ldr := &Loader{
		Model:  s,
		Active: false,
		queue:  make([]LoaderItem, 0),
	}

	return Model{
		ctx:              ctx,
		profile:          profile,
		phase:            runtime.PhaseStartup,
		tiers:            make([]TierView, 0),
		services:         make(map[string]*ServiceState),
		selected:         0,
		keys:             DefaultKeyMap(),
		help:             help.New(),
		event:            event,
		command:          command,
		loader:           ldr,
		viewMode:         ViewModeServices,
		logs:             make([]LogEntry, 0),
		maxLogs:          1000,
		servicesViewport: viewport.New(0, 0),
		logsViewport:     viewport.New(0, 0),
		ready:            false,
		eventChan:        eventChan,
		log:              log,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loader.Model.Tick,
		waitForEventCmd(m.eventChan),
		tickCmd(),
	)
}

func (m Model) getSelectedService() *ServiceState {
	if m.selected < 0 {
		return nil
	}

	idx := 0

	for _, tier := range m.tiers {
		for _, serviceName := range tier.Services {
			if idx == m.selected {
				return m.services[serviceName]
			}

			idx++
		}
	}

	return nil
}

func (m Model) getTotalServices() int {
	total := 0
	for _, tier := range m.tiers {
		total += len(tier.Services)
	}

	return total
}

func (m Model) getReadyServices() int {
	count := 0

	for _, service := range m.services {
		if service.Status == StatusReady {
			count++
		}
	}

	return count
}

func (m Model) getMaxServiceNameLength() int {
	maxLen := 20

	for _, service := range m.services {
		if len(service.Name) > maxLen {
			maxLen = len(service.Name)
		}
	}

	return maxLen
}

func (m Model) calculateScrollOffset() int {
	if m.servicesViewport.Height == 0 {
		return m.servicesViewport.YOffset
	}

	lineNumber := 0
	currentIdx := 0

	for i, tier := range m.tiers {
		tierStartLine := lineNumber

		if i > 0 {
			lineNumber++
		}

		lineNumber++

		serviceIndexInTier := 0

		for range tier.Services {
			if currentIdx == m.selected {
				viewportTop := m.servicesViewport.YOffset
				viewportBottom := viewportTop + m.servicesViewport.Height - 1

				if lineNumber < viewportTop {
					if serviceIndexInTier == 0 {
						return tierStartLine
					}

					return lineNumber
				} else if lineNumber > viewportBottom {
					return lineNumber - m.servicesViewport.Height + 1
				}

				return m.servicesViewport.YOffset
			}

			lineNumber++
			currentIdx++
			serviceIndexInTier++
		}
	}

	return m.servicesViewport.YOffset
}
