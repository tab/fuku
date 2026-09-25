package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

func Test_View_NotReady(t *testing.T) {
	m := Model{}
	m.state.ready = false

	result := m.View()

	assert.Equal(t, tea.NewView("initializing…"), result)
}

func Test_View_RendersWhileShuttingDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	api := &model.Service{ID: "api", Name: "api", Status: model.StatusRunning}
	snapshot := &model.Snapshot{
		Phase:    model.PhaseStopping,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api}}},
		Services: map[string]*model.Service{"api": api},
	}

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))

	loader := &Loader{Model: spinner.New(), Active: true, queue: []LoaderItem{{Service: loaderKeyShutdown, Message: "shutting down…"}}}
	m := Model{loader: loader, registry: mockRegistry}
	m.state.ready = true
	m.state.shuttingDown = true
	m.ui.width = 100
	m.ui.height = 50
	m.ui.layout = terminal.ComputeTableLayout(100-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
	m.ui.help = help.New()
	m.ui.servicesKeys = defaultKeyMap()
	m.ui.servicesViewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(30))
	m.theme = terminal.NewTheme(terminal.AppearanceDark)

	result := m.View()

	assert.NotEmpty(t, result.Content)
	assert.Contains(t, result.Content, "shutting down")
	assert.True(t, result.AltScreen)
}

func Test_RenderTitle_ServicesView(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: false, queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.profile = "default"
	m.ui.width = 100

	title := m.renderTitle()

	assert.Equal(t, "profile • default", title)
}

func Test_RenderTitle_WithActiveLoader(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: true, queue: make([]LoaderItem, 0)}
	loader.Start("api", "starting api…")
	m := Model{loader: loader}
	m.ui.width = 100

	title := m.renderTitle()

	assert.Contains(t, title, "starting api…")
}

func Test_RenderStatus_PhaseColors(t *testing.T) {
	loader := &Loader{Model: spinner.New(), Active: false, queue: make([]LoaderItem, 0)}

	tests := []struct {
		name         string
		before       func() Model
		wantContains string
	}{
		{
			name: "no phase before the first phase change reads as startup",
			before: func() Model {
				return Model{loader: loader, snapshot: &model.Snapshot{Phase: ""}}
			},
			wantContains: "starting…",
		},
		{
			name: "startup phase",
			before: func() Model {
				return Model{loader: loader, snapshot: &model.Snapshot{Phase: model.PhaseStartup}}
			},
			wantContains: "starting…",
		},
		{
			name: "running phase",
			before: func() Model {
				return Model{loader: loader, snapshot: &model.Snapshot{Phase: model.PhaseRunning}}
			},
			wantContains: "running",
		},
		{
			name: "stopping phase",
			before: func() Model {
				return Model{loader: loader, snapshot: &model.Snapshot{Phase: model.PhaseStopping}}
			},
			wantContains: "stopping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			info := m.renderStatus()

			assert.Contains(t, info, tt.wantContains)
		})
	}
}

func Test_RenderStatus_ShowsGlobalCountsWhenFiltering(t *testing.T) {
	m := Model{snapshot: &model.Snapshot{Phase: model.PhaseRunning, Services: map[string]*model.Service{
		"id-svc1": {Status: model.StatusRunning},
		"id-svc2": {Status: model.StatusFailed},
		"id-svc3": {Status: model.StatusRunning},
	}}}
	m.state.filterQuery = "svc1"
	m.state.filteredIDs = []string{"id-svc1"}
	m.theme = terminal.NewTheme(terminal.AppearanceDark)

	result := m.renderStatus()

	assert.Contains(t, result, "2/3 ready")
}

func Test_RenderServices_Empty(t *testing.T) {
	m := Model{snapshot: &model.Snapshot{}}
	m.ui.servicesViewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))

	result := m.renderServices()

	assert.Contains(t, result, "no services configured")
}

func Test_RenderNoWrapAtBreakpoints(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	tests := []struct {
		name     string
		before   func() Model
		service  *model.Service
		rowWidth int
	}{
		{
			name: "72-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 72
				m.ui.layout = terminal.ComputeTableLayout(72-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(72 - terminal.PanelInnerPadding)
				m.state.views = map[string]*serviceView{"id-svc": {Timeline: newTimeline()}}

				return m
			},
			service: &model.Service{
				ID:     "id-svc",
				Name:   "test-service",
				Status: model.StatusRunning,
				Process: model.Process{
					PID:    12345,
					CPU:    99.9,
					Memory: 512 * 1024 * 1024,
				},
			},
			rowWidth: 72 - terminal.PanelInnerPadding,
		},
		{
			name: "104-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 104
				m.ui.layout = terminal.ComputeTableLayout(104-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(104 - terminal.PanelInnerPadding)
				m.state.views = map[string]*serviceView{"id-svc": {Timeline: newTimeline()}}

				return m
			},
			service: &model.Service{
				ID:     "id-svc",
				Name:   "long-service-name-xx",
				Status: model.StatusRunning,
				Process: model.Process{
					PID:    12345,
					CPU:    99.9,
					Memory: 512 * 1024 * 1024,
				},
			},
			rowWidth: 104 - terminal.PanelInnerPadding,
		},
		{
			name: "120-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 120
				m.ui.layout = terminal.ComputeTableLayout(120-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(120 - terminal.PanelInnerPadding)
				m.state.views = map[string]*serviceView{"id-svc": {Timeline: newTimeline()}}

				return m
			},
			service: &model.Service{
				ID:     "id-svc",
				Name:   "long-service-name-xx",
				Status: model.StatusRunning,
				Process: model.Process{
					PID:    12345,
					CPU:    99.9,
					Memory: 512 * 1024 * 1024,
				},
			},
			rowWidth: 120 - terminal.PanelInnerPadding,
		},
		{
			name: "200-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 200
				m.ui.layout = terminal.ComputeTableLayout(200-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(200 - terminal.PanelInnerPadding)
				m.state.views = map[string]*serviceView{"id-svc": {Timeline: newTimeline()}}

				return m
			},
			service: &model.Service{
				ID:     "id-svc",
				Name:   "long-service-name-xx",
				Status: model.StatusRunning,
				Process: model.Process{
					PID:    12345,
					CPU:    99.9,
					Memory: 512 * 1024 * 1024,
				},
			},
			rowWidth: 200 - terminal.PanelInnerPadding,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			header := m.renderColumnHeaders()
			row := m.renderServiceRow(tt.service, false)

			assert.NotContains(t, header, "\n", "header wraps")
			assert.NotContains(t, row, "\n", "service row wraps")
			assert.Equal(t, tt.rowWidth, lipgloss.Width(header), "header width must equal rowWidth")
			assert.Equal(t, tt.rowWidth, lipgloss.Width(row), "service row width must equal rowWidth")
		})
	}
}

func Test_RenderTip_RotatesOverTime(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	tipCount := len(terminal.Tips)
	wrapTick := tipCount * terminal.UITipRotationTicks

	tests := []struct {
		name      string
		before    func() Model
		wantIndex int
	}{
		{
			name: "tick 0 with offset 0",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.tipOffset = 0
				m.ui.tickCounter = 0
				m.ui.showTips = true

				return m
			},
			wantIndex: 0,
		},
		{
			name: "tick 100 with offset 0",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.tipOffset = 0
				m.ui.tickCounter = 100
				m.ui.showTips = true

				return m
			},
			wantIndex: 1,
		},
		{
			name: "tick 0 with offset 3",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.tipOffset = 3
				m.ui.tickCounter = 0
				m.ui.showTips = true

				return m
			},
			wantIndex: 3,
		},
		{
			name: "wraps after all tips",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.tipOffset = 0
				m.ui.tickCounter = wrapTick
				m.ui.showTips = true

				return m
			},
			wantIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.renderTip()

			assert.Equal(t, terminal.Tips[tt.wantIndex].Render(theme), result)
		})
	}
}

func Test_RenderTip_HiddenWhenDisabled(t *testing.T) {
	m := Model{}
	m.ui.showTips = false
	m.ui.tickCounter = 0

	result := m.renderTip()

	assert.Empty(t, result)
}

func Test_RenderAPIDot(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	tests := []struct {
		name   string
		before func() Model
		want   string
	}{
		{
			name: "listening shows solid blue dot",
			before: func() Model {
				return Model{theme: theme, snapshot: &model.Snapshot{API: model.API{Listening: true}}}
			},
			want: theme.APIDotConnected.Render(terminal.IndicatorDot),
		},
		{
			name: "down shows gray dot",
			before: func() Model {
				return Model{theme: theme, snapshot: &model.Snapshot{API: model.API{Listening: false}}}
			},
			want: theme.APIDotDisconnected.Render(terminal.IndicatorDot),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.renderAPIDot()

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_RenderAppStats(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	tests := []struct {
		name   string
		before func() Model
		want   string
	}{
		{
			name: "no stats no api",
			before: func() Model {
				m := Model{theme: theme, snapshot: &model.Snapshot{}}
				m.state.appCPU = 0
				m.state.appMEM = 0

				return m
			},
			want: "",
		},
		{
			name: "stats only",
			before: func() Model {
				m := Model{theme: theme, snapshot: &model.Snapshot{}}
				m.state.appCPU = 1.0
				m.state.appMEM = 100

				return m
			},
			want: theme.PanelMutedStyle.Render("cpu 1.0% • mem 100MB"),
		},
		{
			name: "api listening",
			before: func() Model {
				return Model{theme: theme, snapshot: &model.Snapshot{API: model.API{Listening: true, Address: "127.0.0.1:9876"}}}
			},
			want: theme.APIDotConnected.Render(terminal.IndicatorDot) + " " + theme.PanelMutedStyle.Render("127.0.0.1:9876"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.renderAppStats()

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_RenderServices_FilteredNoMatches(t *testing.T) {
	api := &model.Service{ID: "api", Name: "api", Status: model.StatusRunning}
	m := Model{snapshot: &model.Snapshot{
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api}}},
		Services: map[string]*model.Service{"api": api},
	}}
	m.state.filterQuery = "nonexistent"
	m.state.filteredIDs = []string{}
	m.ui.servicesViewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))

	result := m.renderServices()

	assert.Contains(t, result, "no matching services")
}

func Test_RenderServices_FilteredWithMatches(t *testing.T) {
	api := &model.Service{ID: "api", Name: "api", Status: model.StatusRunning}
	web := &model.Service{ID: "web", Name: "web", Status: model.StatusRunning}
	m := Model{snapshot: &model.Snapshot{
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api, web}}},
		Services: map[string]*model.Service{"api": api, "web": web},
	}}
	m.state.filterQuery = "api"
	m.state.filteredIDs = []string{"api"}
	m.ui.servicesViewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	m.ui.layout = terminal.ComputeTableLayout(80-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
	m.theme = terminal.NewTheme(terminal.AppearanceDark)
	m.updateServicesContent()

	result := m.renderServices()

	assert.Contains(t, result, "api")
	assert.NotContains(t, result, "web")
	assert.NotContains(t, result, "no matching services")
	assert.NotContains(t, result, "no services configured")
}

func Test_RenderFilterBar(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	tests := []struct {
		name   string
		before func() Model
		want   string
	}{
		{
			name: "no filter active and no query",
			before: func() Model {
				m := Model{theme: theme}
				m.state.filterActive = false
				m.state.filterQuery = ""

				return m
			},
			want: "",
		},
		{
			name: "filter active with empty query shows cursor",
			before: func() Model {
				m := Model{theme: theme}
				m.state.filterActive = true
				m.state.filterQuery = ""

				return m
			},
			want: theme.PanelMutedStyle.Render("/ _"),
		},
		{
			name: "filter active with query shows cursor",
			before: func() Model {
				m := Model{theme: theme}
				m.state.filterActive = true
				m.state.filterQuery = "api"

				return m
			},
			want: theme.PanelMutedStyle.Render("/ api_"),
		},
		{
			name: "filter not active but query retained without cursor",
			before: func() Model {
				m := Model{theme: theme}
				m.state.filterActive = false
				m.state.filterQuery = "web"

				return m
			},
			want: theme.PanelMutedStyle.Render("/ web"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.renderFilterBar()

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_View_FilterBarInBottomBorder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	api := &model.Service{ID: "api", Name: "api", Status: model.StatusRunning}
	snapshot := &model.Snapshot{
		Phase:    model.PhaseRunning,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api}}},
		Services: map[string]*model.Service{"api": api},
	}

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))

	loader := &Loader{Model: spinner.New(), Active: false, queue: make([]LoaderItem, 0)}
	m := Model{loader: loader, registry: mockRegistry}
	m.state.ready = true
	m.state.profile = "default"
	m.state.serviceIDs = []string{"api"}
	m.state.filterQuery = "api"
	m.state.filterActive = true
	m.state.filteredIDs = []string{"api"}
	m.ui.width = 100
	m.ui.height = 50
	m.ui.layout = terminal.ComputeTableLayout(100-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
	m.ui.help = help.New()
	m.ui.servicesKeys = defaultKeyMap()
	m.ui.servicesViewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(30))
	m.theme = terminal.NewTheme(terminal.AppearanceDark)

	result := m.View()

	assert.Contains(t, result.Content, "/ api_")
	assert.Contains(t, result.Content, "profile")
}

func Test_RenderBottomLeft_CombinesFilterAndStats(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	m := Model{theme: theme, snapshot: &model.Snapshot{}}
	m.state.filterQuery = "svc"
	m.state.filterActive = false
	m.state.appCPU = 5.0
	m.state.appMEM = 128
	m.ui.width = 100

	result := m.renderBottomLeft()

	assert.Contains(t, result, "/ svc")
	assert.Contains(t, result, "cpu 5.0%")
	assert.Contains(t, result, "mem 128MB")
}

func Test_RenderBottomLeft_FilterOnlyWhenNoStats(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	m := Model{theme: theme, snapshot: &model.Snapshot{}}
	m.state.filterQuery = "web"
	m.state.filterActive = false
	m.ui.width = 100

	result := m.renderBottomLeft()

	assert.Contains(t, result, "/ web")
}

func Test_RenderBottomLeft_StatsOnlyWhenNoFilter(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	m := Model{theme: theme, snapshot: &model.Snapshot{}}
	m.state.appCPU = 2.0
	m.state.appMEM = 64

	result := m.renderBottomLeft()

	assert.Contains(t, result, "cpu 2.0%")
	assert.NotContains(t, result, "/")
}

func Test_RenderFilterBar_TruncatesLongQuery(t *testing.T) {
	m := Model{}
	m.theme = terminal.NewTheme(terminal.AppearanceDark)
	m.ui.width = 40
	m.state.filterActive = true
	m.state.filterQuery = "this-is-a-very-long-service-name-filter-query"

	result := m.renderFilterBar()

	assert.Contains(t, result, "/ this")
	assert.NotContains(t, result, "filter-query")
}

func Test_UpdateServicesContent_UsesFilteredTiers(t *testing.T) {
	api := &model.Service{ID: "api", Name: "api", Status: model.StatusRunning}
	web := &model.Service{ID: "web", Name: "web", Status: model.StatusRunning}
	db := &model.Service{ID: "db", Name: "db", Status: model.StatusRunning}
	m := Model{snapshot: &model.Snapshot{
		Tiers: []*model.Tier{
			{Name: "tier1", Services: []*model.Service{api, web}},
			{Name: "tier2", Services: []*model.Service{db}},
		},
		Services: map[string]*model.Service{"api": api, "web": web, "db": db},
	}}
	m.state.serviceIDs = []string{"api", "web", "db"}
	m.state.filterQuery = "api"
	m.state.filteredIDs = []string{"api"}
	m.ui.width = 100
	m.ui.layout = terminal.ComputeTableLayout(92-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
	m.ui.servicesViewport = viewport.New(viewport.WithWidth(92), viewport.WithHeight(20))
	m.theme = terminal.NewTheme(terminal.AppearanceDark)

	m.updateServicesContent()

	content := m.ui.servicesViewport.View()
	assert.Contains(t, content, "api")
	assert.NotContains(t, content, "db")
}

func Test_RenderVersion(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	tests := []struct {
		name   string
		before func() Model
		want   string
	}{
		{
			name: "no available version renders current in white",
			before: func() Model {
				m := Model{theme: theme}
				m.state.availableVersion = ""

				return m
			},
			want: theme.CurrentVersionStyle.Render("v" + buildinfo.Version),
		},
		{
			name: "with available version renders current white, arrow and latest coral",
			before: func() Model {
				m := Model{theme: theme}
				m.state.availableVersion = "v0.20.0"

				return m
			},
			want: theme.CurrentVersionStyle.Render("v"+buildinfo.Version) + theme.PanelMutedStyle.Render(" - ") + theme.LatestVersionStyle.Render("↑ v0.20.0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.renderVersion()

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_View_AsideClosed_FullWidth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	api := &model.Service{ID: "api", Name: "api", Status: model.StatusRunning}
	snapshot := &model.Snapshot{
		Phase:    model.PhaseRunning,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api}}},
		Services: map[string]*model.Service{"api": api},
	}

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))

	loader := &Loader{Model: spinner.New(), Active: false, queue: make([]LoaderItem, 0)}
	m := Model{loader: loader, registry: mockRegistry}
	m.state.ready = true
	m.state.profile = "default"
	m.state.serviceIDs = []string{"api"}
	m.ui.width = 120
	m.ui.height = 40
	m.ui.layout = terminal.ComputeTableLayout(120-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
	m.ui.help = help.New()
	m.ui.servicesKeys = defaultKeyMap()
	m.ui.servicesViewport = viewport.New(viewport.WithWidth(120-terminal.PanelInnerPadding), viewport.WithHeight(30))
	m.theme = terminal.NewTheme(terminal.AppearanceDark)

	result := m.View()

	firstLine, _, _ := strings.Cut(result.Content, "\n")
	assert.GreaterOrEqual(t, lipgloss.Width(firstLine), 120-terminal.PanelInnerPadding, "main panel should occupy full width when aside is closed")
}

func Test_View_AsideOpen_SplitWidth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	api := &model.Service{ID: "api", Name: "api", Tier: "foundation", Directory: "services/api", Command: "go run main.go", Status: model.StatusRunning}
	snapshot := &model.Snapshot{
		Phase:    model.PhaseRunning,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api}}},
		Services: map[string]*model.Service{"api": api},
	}

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))

	loader := &Loader{Model: spinner.New(), Active: false, queue: make([]LoaderItem, 0)}
	m := Model{loader: loader, registry: mockRegistry, snapshot: snapshot}
	m.ui.asideCache = &asideContentCache{}
	m.state.ready = true
	m.state.profile = "default"
	m.state.serviceIDs = []string{"api"}
	m.state.asideOpen = true
	m.ui.width = 200
	m.ui.height = 40
	m.ui.help = help.New()
	m.ui.servicesKeys = defaultKeyMap()
	m.ui.servicesViewport = viewport.New(viewport.WithWidth(100-terminal.PanelInnerPadding), viewport.WithHeight(30))
	m.theme = terminal.NewTheme(terminal.AppearanceDark)
	m.ui.asideViewport = viewport.New()
	m = m.recomputeLayout()
	m.recomputeViewport()
	m.updateAsideContent()

	result := m.View()

	firstLine, _, _ := strings.Cut(result.Content, "\n")
	assert.GreaterOrEqual(t, lipgloss.Width(firstLine), 200-terminal.PanelInnerPadding, "split view should reach total terminal width")
	assert.Contains(t, result.Content, "services/api", "aside content should appear in split view")
}

func Test_View_AsideOpen_NarrowTerminalHidesAside(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	api := &model.Service{ID: "api", Name: "api", Directory: "services/api", Command: "narrow-only", Status: model.StatusRunning}
	snapshot := &model.Snapshot{
		Phase:    model.PhaseRunning,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api}}},
		Services: map[string]*model.Service{"api": api},
	}

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))

	loader := &Loader{Model: spinner.New(), Active: false, queue: make([]LoaderItem, 0)}
	m := Model{loader: loader, registry: mockRegistry, snapshot: snapshot}
	m.state.ready = true
	m.state.profile = "default"
	m.state.serviceIDs = []string{"api"}
	m.state.asideOpen = true
	m.ui.width = 50
	m.ui.height = 40
	m.ui.help = help.New()
	m.ui.servicesKeys = defaultKeyMap()
	m.ui.servicesViewport = viewport.New(viewport.WithWidth(50-terminal.PanelInnerPadding), viewport.WithHeight(30))
	m.theme = terminal.NewTheme(terminal.AppearanceDark)
	m = m.recomputeLayout()

	result := m.View()

	assert.NotContains(t, result.Content, "narrow-only", "aside config content must not appear when terminal is too narrow")
}

func Test_PanelWidths(t *testing.T) {
	tests := []struct {
		name           string
		before         func() Model
		wantMainWidth  int
		wantAsideWidth int
	}{
		{
			name: "closed full width",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Services: map[string]*model.Service{}}}
				m.ui.width = 200
				m.state.asideOpen = false

				return m
			},
			wantMainWidth:  200,
			wantAsideWidth: 0,
		},
		{
			name: "open with short names floors main at AsideMinMainWidth and donates rest to aside",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Services: map[string]*model.Service{
					"api": {ID: "api", Name: "api"},
					"web": {ID: "web", Name: "web"},
				}}}
				m.ui.width = 200
				m.state.asideOpen = true

				return m
			},
			wantMainWidth:  terminal.AsideMinMainWidth,
			wantAsideWidth: 200 - terminal.AsideMinMainWidth,
		},
		{
			name: "open with medium name grows main to fit (frontend-api → 21 < min 24)",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Services: map[string]*model.Service{
					"frontend-api": {ID: "frontend-api", Name: "frontend-api"},
				}}}
				m.ui.width = 200
				m.state.asideOpen = true

				return m
			},
			wantMainWidth:  terminal.AsideMinMainWidth,
			wantAsideWidth: 200 - terminal.AsideMinMainWidth,
		},
		{
			name: "open with long name caps at ServiceNameWidthMedium so the aside is protected from outliers",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Services: map[string]*model.Service{
					"action-confirmation-management-service": {ID: "action-confirmation-management-service", Name: "action-confirmation-management-service"},
				}}}
				m.ui.width = 200
				m.state.asideOpen = true

				return m
			},
			wantMainWidth:  terminal.ServiceNameWidthMedium + terminal.IndicatorColumnWidth + terminal.ServiceNameTrailingGap + terminal.StatusCompactWidth + terminal.PanelInnerPadding + terminal.RowHorizontalPadding,
			wantAsideWidth: 200 - (terminal.ServiceNameWidthMedium + terminal.IndicatorColumnWidth + terminal.ServiceNameTrailingGap + terminal.StatusCompactWidth + terminal.PanelInnerPadding + terminal.RowHorizontalPadding),
		},
		{
			name: "open but too narrow falls back to full width",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{Services: map[string]*model.Service{
					"api": {ID: "api", Name: "api"},
				}}}
				m.ui.width = 50
				m.state.asideOpen = true

				return m
			},
			wantMainWidth:  50,
			wantAsideWidth: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			mainWidth, asideWidth := m.panelWidths()

			assert.Equal(t, tt.wantMainWidth, mainWidth)
			assert.Equal(t, tt.wantAsideWidth, asideWidth)
		})
	}
}

func Test_AsideVisible(t *testing.T) {
	tests := []struct {
		name   string
		before func() Model
		want   bool
	}{
		{
			name: "closed not visible",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{}}
				m.ui.width = 200
				m.state.asideOpen = false

				return m
			},
			want: false,
		},
		{
			name: "open and wide enough visible",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{}}
				m.ui.width = 200
				m.state.asideOpen = true

				return m
			},
			want: true,
		},
		{
			name: "open but too narrow not visible",
			before: func() Model {
				m := Model{snapshot: &model.Snapshot{}}
				m.ui.width = 50
				m.state.asideOpen = true

				return m
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.asideVisible()

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_UpdateServicesContent_EmptyFilterClearsViewport(t *testing.T) {
	api := &model.Service{ID: "api", Name: "api", Status: model.StatusRunning}
	m := Model{snapshot: &model.Snapshot{
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api}}},
		Services: map[string]*model.Service{"api": api},
	}}
	m.state.filterQuery = "nonexistent"
	m.state.filteredIDs = []string{}
	m.ui.servicesViewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))

	m.updateServicesContent()

	content := m.ui.servicesViewport.View()
	assert.NotContains(t, content, "api")
	assert.NotContains(t, content, "tier1")
}

func Test_metricLabels(t *testing.T) {
	tests := []struct {
		name          string
		metricColumns int
		expected      []string
	}{
		{
			name:          "four columns show every label",
			metricColumns: 4,
			expected:      []string{"cpu", "mem", "pid", "uptime"},
		},
		{
			name:          "more columns than labels show every label",
			metricColumns: 6,
			expected:      []string{"cpu", "mem", "pid", "uptime"},
		},
		{
			name:          "two columns show the leading labels",
			metricColumns: 2,
			expected:      []string{"cpu", "mem"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := metricLabels(tt.metricColumns)

			assert.Equal(t, tt.expected, result)
		})
	}
}
