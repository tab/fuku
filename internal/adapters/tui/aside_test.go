package tui

import (
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"

	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
)

func Test_AsideContent(t *testing.T) {
	m := Model{theme: terminal.NewTheme(terminal.AppearanceDark)}

	tests := []struct {
		name        string
		service     *model.Service
		wantContain []string
		wantMissing []string
	}{
		{
			name:    "nil service shows empty state",
			service: nil,
			wantContain: []string{
				"no service selected",
			},
		},
		{
			name: "service without optional config renders the meta and logs cards only",
			service: &model.Service{
				ID:        "api",
				Name:      "api",
				Tier:      "foundation",
				LogOutput: []string{"stdout", "stderr"},
				Status:    model.StatusRunning,
			},
			wantContain: []string{
				"meta",
				"tier", "foundation",
				"output", "stdout, stderr",
			},
			wantMissing: []string{
				"dir",
				"address", "pattern",
				"include",
			},
		},
		{
			name: "service with full config renders all cards",
			service: &model.Service{
				ID:        "api",
				Name:      "api",
				Directory: "services/api",
				Command:   "go run cmd/main.go",
				Tier:      "foundation",
				Readiness: &model.Readiness{
					Type: model.ReadinessHTTP,
					URL:  "http://localhost:8080/healthz",
				},
				LogOutput: []string{"stdout", "stderr"},
				Watch: &model.Watch{
					Include:  []string{"**/*.go"},
					Ignore:   []string{"vendor/**"},
					Shared:   []string{"shared/**"},
					Debounce: 250 * time.Millisecond,
				},
				Status: model.StatusRunning,
			},
			wantContain: []string{
				"go run cmd/main.go",
				"meta",
				"tier", "foundation",
				"dir", "services/api",
				"command",
				"readiness",
				"type", "http",
				"url", "http://localhost:8080/healthz",
				"logs",
				"output", "stdout, stderr",
				"watch",
				"include", "**/*.go",
				"ignore", "vendor/**",
				"shared", "shared/**",
				"debounce", "250ms",
			},
		},
		{
			name: "service with partial config omits empty cards",
			service: &model.Service{
				ID:        "db",
				Name:      "db",
				Command:   "make run",
				Directory: "services/db",
				Tier:      "foundation",
				LogOutput: []string{"stdout"},
				Status:    model.StatusStarting,
			},
			wantContain: []string{
				"make run",
				"meta",
				"tier", "foundation",
				"dir", "services/db",
				"command",
				"output", "stdout",
			},
			wantMissing: []string{
				"address", "pattern",
				"include",
			},
		},
		{
			name: "tcp readiness shows address not url",
			service: &model.Service{
				ID:        "redis",
				Name:      "redis",
				Directory: "services/redis",
				Readiness: &model.Readiness{
					Type:    model.ReadinessTCP,
					Address: "localhost:6379",
				},
				Status: model.StatusRunning,
			},
			wantContain: []string{
				"readiness",
				"type", "tcp",
				"address", "localhost:6379",
			},
			wantMissing: []string{
				"url",
				"pattern",
			},
		},
		{
			name: "service with an error renders the error card",
			service: &model.Service{
				ID:      "api",
				Name:    "api",
				Command: "go run .",
				Status:  model.StatusFailed,
				Error:   "exit status 1",
			},
			wantContain: []string{
				"error",
				"reason", "exit status 1",
			},
		},
		{
			name: "empty watch struct does not produce watch card",
			service: &model.Service{
				ID:        "web",
				Name:      "web",
				Directory: "services/web",
				Watch:     &model.Watch{},
				Status:    model.StatusRunning,
			},
			wantMissing: []string{
				"include",
				"ignore",
				"shared",
				"debounce",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.asideContent(tt.service, 80)

			for _, want := range tt.wantContain {
				assert.Contains(t, result, want)
			}

			for _, miss := range tt.wantMissing {
				assert.NotContains(t, result, miss)
			}
		})
	}
}

func Test_RenderAsideLines(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	tests := []struct {
		name        string
		before      func() Model
		width       int
		height      int
		wantContain []string
	}{
		{
			name: "empty state shows placeholder message",
			before: func() Model {
				m := Model{theme: theme, snapshot: &model.Snapshot{Services: map[string]*model.Service{}}}
				m.ui.asideCache = &asideContentCache{}
				m.state.serviceIDs = []string{}
				m.state.asideOpen = true
				m.ui.asideViewport.SetWidth(40 - terminal.PanelInnerPadding)
				m.ui.asideViewport.SetHeight(10 - terminal.PanelBorderHeight)
				m.updateAsideContent()

				return m
			},
			width:  40,
			height: 10,
			wantContain: []string{
				"no service selected",
			},
		},
		{
			name: "selected service renders dir and custom command",
			before: func() Model {
				m := Model{theme: theme, snapshot: &model.Snapshot{Services: map[string]*model.Service{
					"api": {ID: "api", Name: "api", Directory: "services/api", Command: "go run main.go", Tier: "foundation", Status: model.StatusRunning},
				}}}
				m.ui.asideCache = &asideContentCache{}
				m.state.serviceIDs = []string{"api"}
				m.state.asideOpen = true
				m.ui.asideViewport.SetWidth(60 - terminal.PanelInnerPadding)
				m.ui.asideViewport.SetHeight(15 - terminal.PanelBorderHeight)
				m.updateAsideContent()

				return m
			},
			width:  60,
			height: 15,
			wantContain: []string{
				"services/api",
				"go run main.go",
			},
		},
		{
			name: "projected default command is shown as is",
			before: func() Model {
				m := Model{theme: theme, snapshot: &model.Snapshot{Services: map[string]*model.Service{
					"api": {ID: "api", Name: "api", Command: "make run", Directory: "services/api", Status: model.StatusRunning},
				}}}
				m.ui.asideCache = &asideContentCache{}
				m.state.serviceIDs = []string{"api"}
				m.state.asideOpen = true
				m.ui.asideViewport.SetWidth(60 - terminal.PanelInnerPadding)
				m.ui.asideViewport.SetHeight(15 - terminal.PanelBorderHeight)
				m.updateAsideContent()

				return m
			},
			width:  60,
			height: 15,
			wantContain: []string{
				"make run",
			},
		},
		{
			name: "output pads to exact height",
			before: func() Model {
				m := Model{theme: theme, snapshot: &model.Snapshot{}}
				m.ui.asideCache = &asideContentCache{}
				m.state.serviceIDs = []string{}
				m.state.asideOpen = true
				m.ui.asideViewport.SetWidth(40 - terminal.PanelInnerPadding)
				m.ui.asideViewport.SetHeight(12 - terminal.PanelBorderHeight)
				m.updateAsideContent()

				return m
			},
			width:  40,
			height: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := strings.Join(m.renderAsideLines(tt.width, tt.height), "\n")

			assert.NotEmpty(t, result)

			for _, want := range tt.wantContain {
				assert.Contains(t, result, want)
			}

			assert.Len(t, strings.Split(result, "\n"), tt.height)
		})
	}
}

func Test_UpdateAsideContent_Cache(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	log := slog.New(slog.DiscardHandler)

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	api := &model.Service{ID: "id-api", Name: "api", Status: model.StatusRunning, LifecycleAt: now.Add(-time.Minute)}
	web := &model.Service{ID: "id-web", Name: "web", Status: model.StatusStopped, LifecycleAt: now.Add(-time.Hour)}
	snapshot := &model.Snapshot{
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api, web}}},
		Services: map[string]*model.Service{"id-api": api, "id-web": web},
	}

	subject := NewModel(t.Context(), ModelParams{Profile: "default", Theme: theme, Logger: log})
	subject.snapshot = snapshot
	subject.state.serviceIDs = []string{"id-api", "id-web"}
	subject.state.asideOpen = true
	subject.state.asideTab = AsideTabHealth
	subject.state.now = now
	subject.ui.asideViewport.SetWidth(60)
	subject.ui.asideViewport.SetHeight(20)

	primed := func() Model {
		m := subject
		m.ui.asideCache = &asideContentCache{}
		m.updateAsideContent()

		return m
	}

	tests := []struct {
		name   string
		before func() (Model, []string)
		cached bool
	}{
		{
			name: "an unchanged state keeps the cached lines",
			before: func() (Model, []string) {
				m := primed()

				return m, m.ui.asideLines
			},
			cached: true,
		},
		{
			name: "a later instant within the same second keeps the cached lines",
			before: func() (Model, []string) {
				m := primed()
				m.state.now = now.Add(500 * time.Millisecond)

				return m, m.ui.asideLines
			},
			cached: true,
		},
		{
			name: "a tab change rebuilds the lines",
			before: func() (Model, []string) {
				m := primed()
				m.state.asideTab = AsideTabConfig

				return m, m.ui.asideLines
			},
			cached: false,
		},
		{
			name: "a selection change rebuilds the lines",
			before: func() (Model, []string) {
				m := primed()
				m.state.selected = 1

				return m, m.ui.asideLines
			},
			cached: false,
		},
		{
			name: "the next second rebuilds the lines",
			before: func() (Model, []string) {
				m := primed()
				m.state.now = now.Add(time.Second)

				return m, m.ui.asideLines
			},
			cached: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, lines := tt.before()

			m.updateAsideContent()

			assert.Equal(t, tt.cached, &lines[0] == &m.ui.asideLines[0])
			assert.Equal(t, tt.cached, slices.Equal(lines, m.ui.asideLines))
		})
	}
}

func Test_AsideRow_TruncatesLongValue(t *testing.T) {
	m := Model{theme: terminal.NewTheme(terminal.AppearanceDark)}

	tests := []struct {
		name       string
		row        cardRow
		labelWidth int
		available  int
		wantSubstr string
	}{
		{
			name:       "value fits without truncation",
			row:        cardRow{label: "dir", value: "svc/api"},
			labelWidth: 6,
			available:  40,
			wantSubstr: "svc/api",
		},
		{
			name:       "long value is truncated with ellipsis",
			row:        cardRow{label: "dir", value: "services/very-long-service-name-that-overflows-the-aside"},
			labelWidth: 6,
			available:  20,
			wantSubstr: "…",
		},
		{
			name:       "no room for value returns label only",
			row:        cardRow{label: "dir", value: "anything"},
			labelWidth: 6,
			available:  6,
			wantSubstr: "dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.asideRow(tt.row, tt.labelWidth, tt.available)

			assert.Contains(t, result, tt.wantSubstr)
			assert.LessOrEqual(t, lipgloss.Width(result), tt.available, "row must not exceed available width")
		})
	}
}

func Test_AsideRendering_NoOverflowForLongValues(t *testing.T) {
	m := Model{theme: terminal.NewTheme(terminal.AppearanceDark)}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"api": {
			ID:        "api",
			Name:      "api",
			Directory: "services/extremely-long-directory-path-that-would-otherwise-overflow",
			Command:   "go run cmd/main.go --some=long --flag=values --foo=bar --baz=qux",
			Status:    model.StatusRunning,
		},
	}}
	m.state.serviceIDs = []string{"api"}

	width := 40

	lines := m.renderAsideLines(width, 15)

	for _, line := range lines {
		assert.LessOrEqual(t, lipgloss.Width(line), width, "every aside line must fit inside the panel width")
	}
}

func Test_NextAsideTab(t *testing.T) {
	tests := []struct {
		name string
		from AsideTab
		want AsideTab
	}{
		{
			name: "config -> env",
			from: AsideTabConfig,
			want: AsideTabEnv,
		},
		{
			name: "env -> health",
			from: AsideTabEnv,
			want: AsideTabHealth,
		},
		{
			name: "health wraps to config",
			from: AsideTabHealth,
			want: AsideTabConfig,
		},
		{
			name: "unknown tab resets to first tab",
			from: AsideTab(""),
			want: AsideTabConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nextAsideTab(tt.from)

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_PrevAsideTab(t *testing.T) {
	tests := []struct {
		name string
		from AsideTab
		want AsideTab
	}{
		{
			name: "env -> config",
			from: AsideTabEnv,
			want: AsideTabConfig,
		},
		{
			name: "health -> env",
			from: AsideTabHealth,
			want: AsideTabEnv,
		},
		{
			name: "config wraps to health",
			from: AsideTabConfig,
			want: AsideTabHealth,
		},
		{
			name: "unknown tab resets to first tab",
			from: AsideTab(""),
			want: AsideTabConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := prevAsideTab(tt.from)

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_AsideScrollIndicator(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	tests := []struct {
		name   string
		before func() Model
		want   string
	}{
		{
			name: "content fits inside viewport returns empty",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 2))

				return m
			},
			want: "",
		},
		{
			name: "content overflows and at top returns percent",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))

				return m
			},
			want: theme.PanelMutedStyle.Render("0%"),
		},
		{
			name: "content overflows and at bottom returns percent",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
				m.ui.asideViewport.SetYOffset(50)

				return m
			},
			want: theme.PanelMutedStyle.Render("100%"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			got := m.asideScrollIndicator()

			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_AsideBorderTabs(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	muted := theme.PanelMutedStyle.Render
	active := theme.StatusRunningStyle.Render
	separator := muted(asideTabSeparator)

	tests := []struct {
		name     string
		before   func() Model
		expected string
	}{
		{
			name: "config tab is active",
			before: func() Model {
				m := Model{theme: theme}
				m.state.asideTab = AsideTabConfig

				return m
			},
			expected: active("config") + separator + muted("env") + separator + muted("health"),
		},
		{
			name: "env tab is active",
			before: func() Model {
				m := Model{theme: theme}
				m.state.asideTab = AsideTabEnv

				return m
			},
			expected: muted("config") + separator + active("env") + separator + muted("health"),
		},
		{
			name: "health tab is active",
			before: func() Model {
				m := Model{theme: theme}
				m.state.asideTab = AsideTabHealth

				return m
			},
			expected: muted("config") + separator + muted("env") + separator + active("health"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.asideBorderTabs()

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_AsideTabIndex(t *testing.T) {
	tests := []struct {
		name string
		tab  AsideTab
		want int
	}{
		{
			name: "config is at index 0",
			tab:  AsideTabConfig,
			want: 0,
		},
		{
			name: "env is at index 1",
			tab:  AsideTabEnv,
			want: 1,
		},
		{
			name: "health is at index 2",
			tab:  AsideTabHealth,
			want: 2,
		},
		{
			name: "unknown tab returns -1",
			tab:  AsideTab("bogus"),
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := asideTabIndex(tt.tab)

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_StatusStyle(t *testing.T) {
	m := Model{theme: terminal.NewTheme(terminal.AppearanceDark)}

	tests := []struct {
		name   string
		status model.Status
		want   lipgloss.Style
	}{
		{
			name:   "pending uses pending style",
			status: model.StatusPending,
			want:   m.theme.StatusPendingStyle,
		},
		{
			name:   "running uses running style",
			status: model.StatusRunning,
			want:   m.theme.StatusRunningStyle,
		},
		{
			name:   "starting uses starting style",
			status: model.StatusStarting,
			want:   m.theme.StatusStartingStyle,
		},
		{
			name:   "restarting uses starting style",
			status: model.StatusRestarting,
			want:   m.theme.StatusStartingStyle,
		},
		{
			name:   "stopping uses starting style",
			status: model.StatusStopping,
			want:   m.theme.StatusStartingStyle,
		},
		{
			name:   "failed uses failed style",
			status: model.StatusFailed,
			want:   m.theme.StatusFailedStyle,
		},
		{
			name:   "stopped uses stopped style",
			status: model.StatusStopped,
			want:   m.theme.StatusStoppedStyle,
		},
		{
			name:   "unknown uses muted style",
			status: model.Status("bogus"),
			want:   m.theme.PanelMutedStyle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.statusStyle(tt.status)

			assert.Equal(t, tt.want, result)
		})
	}
}
