package tui

import (
	"context"
	"math/rand/v2"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"fuku/internal/adapters/resources"
	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
)

// serviceView is what the view keeps about one service: its animations, startup accounting and the facts it last saw
type serviceView struct {
	Status         model.Status
	AttemptedAt    time.Time
	StartupSampled int
	StartupActive  bool
	Blink          *terminal.Blink
	Timeline       *Timeline
}

// Monitor reads the resource usage of the fuku process
type Monitor interface {
	GetStats(ctx context.Context, pid int) (resources.Stats, error)
}

// Registry runs a read of the runtime read model the view renders
type Registry interface {
	Read(fn func(*model.Snapshot))
}

// Environment reads the merged .env entries cached for a service
type Environment interface {
	Env(id string) []model.Env
}

// Logger is the logging surface the services view writes through
type Logger interface {
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
}

// Model represents the Bubble Tea model for the services UI
type Model struct {
	ctx           context.Context
	retryAttempts int
	retryBackoff  time.Duration
	control       Control
	registry      Registry
	monitor       Monitor
	environment   Environment
	theme         terminal.Theme

	loader   *Loader
	snapshot *model.Snapshot // set inside the Read callback, cleared by Update before it returns the model

	state struct {
		profile      string
		resolved     bool
		views        map[string]*serviceView
		serviceIDs   []string
		restarting   map[string]bool
		selected     int
		ready        bool
		shuttingDown bool
		appCPU       float64
		appMEM       float64
		now          time.Time

		filterQuery            string
		filterActive           bool
		filteredIDs            []string
		preFilterSelectedID    string
		lastFilteredSelectedID string

		availableVersion string

		asideOpen    bool
		asideFocused bool
		asideTab     AsideTab
	}

	ui struct {
		height           int
		width            int
		layout           terminal.TableLayout
		servicesKeys     KeyMap
		tickCounter      int
		showTips         bool
		tipOffset        int
		help             help.Model
		servicesViewport viewport.Model
		asideViewport    viewport.Model
		asideLines       []string
		asideCache       *asideContentCache
	}

	log Logger
}

// ModelParams contains the dependencies of the services UI model
type ModelParams struct {
	Profile       string
	RetryAttempts int
	RetryBackoff  time.Duration
	Control       Control
	Registry      Registry
	Monitor       Monitor
	Environment   Environment
	Theme         terminal.Theme
	Logger        Logger
}

// NewModel creates the services UI model (bus messages reach it as EventMsg values sent to the program)
func NewModel(ctx context.Context, params ModelParams) Model {
	m := Model{
		ctx:           ctx,
		retryAttempts: params.RetryAttempts,
		retryBackoff:  params.RetryBackoff,
		control:       params.Control,
		registry:      params.Registry,
		monitor:       params.Monitor,
		environment:   params.Environment,
		theme:         params.Theme,
		loader:        NewLoader(),
		log:           params.Logger,
	}

	m.state.profile = params.Profile
	m.state.views = make(map[string]*serviceView)
	m.state.restarting = make(map[string]bool)
	m.state.asideTab = AsideTabConfig

	m.ui.servicesKeys = defaultKeyMap()
	m.ui.showTips = true
	//nolint:gosec // not security-critical
	m.ui.tipOffset = rand.IntN(len(terminal.Tips))
	m.ui.help = help.New()
	m.ui.help.Styles = help.DefaultStyles(params.Theme.Appearance == terminal.AppearanceDark)
	m.ui.servicesViewport = viewport.New()
	m.ui.asideViewport = viewport.New()
	m.ui.asideCache = &asideContentCache{}

	return m
}

// asideContentCache holds the key of the last aside build (a pointer so value receivers can write it)
type asideContentCache struct {
	key string
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loader.Model.Tick,
		tickCmd(),
		tea.RequestBackgroundColor,
	)
}

// isFiltering returns true when an effective filter query is applied
func (m Model) isFiltering() bool {
	return normalizeQuery(m.state.filterQuery) != ""
}

// activeServiceIDs returns filteredIDs when filtering, otherwise serviceIDs
func (m Model) activeServiceIDs() []string {
	if m.isFiltering() {
		return m.state.filteredIDs
	}

	return m.state.serviceIDs
}

// getSelectedService returns the currently selected service of the read model
func (m Model) getSelectedService() *model.Service {
	ids := m.activeServiceIDs()
	if m.state.selected < 0 || m.state.selected >= len(ids) {
		return nil
	}

	return m.snapshot.Services[ids[m.state.selected]]
}

// activeTiers returns the tiers of the read model, narrowed to the matching services when filtering
func (m Model) activeTiers() []*model.Tier {
	if m.isFiltering() {
		return filterTiers(m.state.filterQuery, m.snapshot.Tiers)
	}

	return m.snapshot.Tiers
}

// calculateScrollOffset calculates the scroll offset to ensure the selected service is visible
func (m Model) calculateScrollOffset() int {
	if m.ui.servicesViewport.Height() == 0 {
		return m.ui.servicesViewport.YOffset()
	}

	lineNumber := 1
	currentIdx := 0

	for i, tier := range m.activeTiers() {
		tierStartLine := lineNumber

		if i > 0 {
			lineNumber++
		}

		lineNumber++

		serviceIndexInTier := 0

		for range tier.Services {
			if currentIdx != m.state.selected {
				lineNumber++
				currentIdx++
				serviceIndexInTier++

				continue
			}

			viewportTop := m.ui.servicesViewport.YOffset()
			viewportBottom := viewportTop + m.ui.servicesViewport.Height() - 1

			switch {
			case lineNumber < viewportTop && serviceIndexInTier == 0:
				return tierStartLine
			case lineNumber < viewportTop:
				return lineNumber
			case lineNumber > viewportBottom:
				return lineNumber - m.ui.servicesViewport.Height() + 1
			default:
				return m.ui.servicesViewport.YOffset()
			}
		}
	}

	return m.ui.servicesViewport.YOffset()
}

// updateServicesContent rebuilds the services viewport content
func (m *Model) updateServicesContent() {
	tiers := m.activeTiers()

	if len(tiers) == 0 {
		m.ui.servicesViewport.SetContent("")

		return
	}

	sections := make([]string, 0, len(tiers)+1)
	if header := m.renderColumnHeaders(); header != "" {
		sections = append(sections, header)
	}

	currentIdx := 0

	for _, tier := range tiers {
		sections = append(sections, m.renderTier(tier, &currentIdx))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	if m.asideVisible() {
		content = terminal.ContentTopMarginStyle.Render(content)
	}

	m.ui.servicesViewport.SetContent(content)
}
