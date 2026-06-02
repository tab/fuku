package services

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/dotenv"
	"fuku/internal/app/ui/components"
	"fuku/internal/config"
)

func Test_AsideContent(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.Config
		service     *ServiceState
		wantContain []string
		wantMissing []string
	}{
		{
			name:    "nil service shows empty state",
			cfg:     nil,
			service: nil,
			wantContain: []string{
				"no service selected",
			},
		},
		{
			name: "service without config entry renders meta card only",
			cfg: &config.Config{
				Services: map[string]*config.Service{},
			},
			service: &ServiceState{
				ID:     "api",
				Name:   "api",
				Tier:   "foundation",
				Status: StatusRunning,
			},
			wantContain: []string{
				"meta",
				"tier", "foundation",
			},
			wantMissing: []string{
				"dir",
				"address", "pattern",
				"output",
				"include",
			},
		},
		{
			name: "config is looked up by name, not id",
			cfg: &config.Config{
				Services: map[string]*config.Service{
					"api": {
						Dir:     "services/api",
						Command: "go run cmd/main.go",
					},
				},
			},
			service: &ServiceState{
				ID:     "9a7b4c10-2e3f-4d5a-8b1c-7e6f0a1b2c3d",
				Name:   "api",
				Tier:   "foundation",
				Status: StatusRunning,
			},
			wantContain: []string{
				"services/api",
				"go run cmd/main.go",
			},
		},
		{
			name: "service with full config renders all cards",
			cfg: &config.Config{
				Services: map[string]*config.Service{
					"api": {
						Dir:     "services/api",
						Command: "go run cmd/main.go",
						Tier:    "foundation",
						Readiness: &config.Readiness{
							Type: "http",
							URL:  "http://localhost:8080/healthz",
						},
						Logs: &config.Logs{
							Output: []string{"stdout", "stderr"},
						},
						Watch: &config.Watch{
							Include:  []string{"**/*.go"},
							Ignore:   []string{"vendor/**"},
							Shared:   []string{"shared/**"},
							Debounce: 250 * time.Millisecond,
						},
					},
				},
			},
			service: &ServiceState{
				ID:     "api",
				Name:   "api",
				Tier:   "foundation",
				Status: StatusRunning,
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
			cfg: &config.Config{
				Services: map[string]*config.Service{
					"db": {
						Dir:  "services/db",
						Tier: "foundation",
					},
				},
			},
			service: &ServiceState{
				ID:     "db",
				Name:   "db",
				Tier:   "foundation",
				Status: StatusStarting,
			},
			wantContain: []string{
				config.DefaultServiceCommand,
				"meta",
				"tier", "foundation",
				"dir", "services/db",
				"command",
			},
			wantMissing: []string{
				"address", "pattern",
				"output",
				"include",
			},
		},
		{
			name: "tcp readiness shows address not url",
			cfg: &config.Config{
				Services: map[string]*config.Service{
					"redis": {
						Dir: "services/redis",
						Readiness: &config.Readiness{
							Type:    "tcp",
							Address: "localhost:6379",
						},
					},
				},
			},
			service: &ServiceState{
				ID:     "redis",
				Name:   "redis",
				Status: StatusRunning,
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
			name: "empty watch struct does not produce watch card",
			cfg: &config.Config{
				Services: map[string]*config.Service{
					"web": {
						Dir:   "services/web",
						Watch: &config.Watch{},
					},
				},
			},
			service: &ServiceState{
				ID:     "web",
				Name:   "web",
				Status: StatusRunning,
			},
			wantMissing: []string{
				"include",
				"ignore",
				"shared",
				"debounce",
			},
		},
		{
			name: "nil config falls back to meta card only",
			cfg:  nil,
			service: &ServiceState{
				ID:     "api",
				Name:   "api",
				Tier:   "foundation",
				Status: StatusRunning,
			},
			wantContain: []string{
				"meta",
				"tier", "foundation",
			},
			wantMissing: []string{
				"dir",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{theme: components.DefaultTheme(), cfg: tt.cfg}

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
	tests := []struct {
		name        string
		cfg         *config.Config
		services    map[string]*ServiceState
		serviceIDs  []string
		selected    int
		width       int
		height      int
		wantContain []string
		wantLines   int
	}{
		{
			name:       "empty state shows placeholder message",
			services:   map[string]*ServiceState{},
			serviceIDs: []string{},
			width:      40,
			height:     10,
			wantContain: []string{
				"no service selected",
			},
		},
		{
			name: "selected service renders dir and custom command",
			cfg: &config.Config{
				Services: map[string]*config.Service{
					"api": {Dir: "services/api", Command: "go run main.go", Tier: "foundation"},
				},
			},
			services: map[string]*ServiceState{
				"api": {ID: "api", Name: "api", Tier: "foundation", Status: StatusRunning},
			},
			serviceIDs: []string{"api"},
			width:      60,
			height:     15,
			wantContain: []string{
				"services/api",
				"go run main.go",
			},
		},
		{
			name: "empty command falls back to default",
			cfg: &config.Config{
				Services: map[string]*config.Service{
					"api": {Dir: "services/api"},
				},
			},
			services: map[string]*ServiceState{
				"api": {ID: "api", Name: "api", Status: StatusRunning},
			},
			serviceIDs: []string{"api"},
			width:      60,
			height:     15,
			wantContain: []string{
				config.DefaultServiceCommand,
			},
		},
		{
			name:       "output pads to exact height",
			serviceIDs: []string{},
			width:      40,
			height:     12,
			wantLines:  12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{theme: components.DefaultTheme(), cfg: tt.cfg}
			m.state.services = tt.services
			m.state.serviceIDs = tt.serviceIDs
			m.state.selected = tt.selected
			m.state.asideOpen = true
			m.ui.asideViewport.SetWidth(max(tt.width-components.PanelInnerPadding, 1))
			m.ui.asideViewport.SetHeight(max(tt.height-components.PanelBorderHeight, 1))
			m.updateAsideContent()

			result := strings.Join(m.renderAsideLines(tt.width, tt.height), "\n")

			assert.NotEmpty(t, result)

			for _, want := range tt.wantContain {
				assert.Contains(t, result, want)
			}

			if tt.wantLines > 0 {
				assert.Len(t, strings.Split(result, "\n"), tt.wantLines)
			}
		})
	}
}

func Test_AsideRow_TruncatesLongValue(t *testing.T) {
	tests := []struct {
		name       string
		label      string
		value      string
		labelWidth int
		available  int
		wantSubstr string
	}{
		{
			name:       "value fits without truncation",
			label:      "dir",
			value:      "svc/api",
			labelWidth: 6,
			available:  40,
			wantSubstr: "svc/api",
		},
		{
			name:       "long value is truncated with ellipsis",
			label:      "dir",
			value:      "services/very-long-service-name-that-overflows-the-aside",
			labelWidth: 6,
			available:  20,
			wantSubstr: "…",
		},
		{
			name:       "no room for value returns label only",
			label:      "dir",
			value:      "anything",
			labelWidth: 6,
			available:  6,
			wantSubstr: "dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{theme: components.DefaultTheme()}

			row := m.asideRow(cardRow{label: tt.label, value: tt.value}, tt.labelWidth, tt.available)
			assert.Contains(t, row, tt.wantSubstr)
			assert.LessOrEqual(t, lipgloss.Width(row), tt.available, "row must not exceed available width")
		})
	}
}

func Test_AsideRendering_NoOverflowForLongValues(t *testing.T) {
	m := Model{theme: components.DefaultTheme()}
	m.cfg = &config.Config{
		Services: map[string]*config.Service{
			"api": {
				Dir:     "services/extremely-long-directory-path-that-would-otherwise-overflow",
				Command: "go run cmd/main.go --some=long --flag=values --foo=bar --baz=qux",
			},
		},
	}
	m.state.services = map[string]*ServiceState{
		"api": {ID: "api", Name: "api", Status: StatusRunning},
	}
	m.state.serviceIDs = []string{"api"}

	width := 40
	lines := m.renderAsideLines(width, 15)

	for _, line := range lines {
		assert.LessOrEqual(t, lipgloss.Width(line), width, "every aside line must fit inside the panel width")
	}
}

func Test_AsideHealthTab(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		cfg         *config.Config
		service     *ServiceState
		wantContain []string
		wantMissing []string
	}{
		{
			name: "http readiness, running pid, retry config, last event",
			cfg: &config.Config{
				Retry: config.Retry{Attempts: 3, Backoff: 500 * time.Millisecond},
				Services: map[string]*config.Service{
					"api": {
						Dir: "services/api",
						Readiness: &config.Readiness{
							Type:     config.TypeHTTP,
							URL:      "http://localhost:8080/health",
							Interval: 500 * time.Millisecond,
							Timeout:  30 * time.Second,
						},
					},
				},
			},
			service: &ServiceState{
				ID:          "api",
				Name:        "api",
				Status:      StatusRunning,
				PID:         76758,
				StartTime:   now.Add(-4*time.Minute - 9*time.Second),
				LifecycleAt: now.Add(-4*time.Minute - 9*time.Second),
			},
			wantContain: []string{
				"probe",
				"type", "http",
				"url", "http://localhost:8080/health",
				"interval", "500ms",
				"timeout", "30s",
				"process",
				"pid", "76758",
				"uptime", "04:09",
				"retry",
				"attempts", "3",
				"backoff", "500ms",
				"status",
				"state", "running",
				"duration", "04:09",
			},
		},
		{
			name: "tcp readiness uses address",
			cfg: &config.Config{
				Services: map[string]*config.Service{
					"redis": {
						Dir: "services/redis",
						Readiness: &config.Readiness{
							Type:    config.TypeTCP,
							Address: "localhost:6379",
						},
					},
				},
			},
			service: &ServiceState{
				ID:     "redis",
				Name:   "redis",
				Status: StatusRunning,
			},
			wantContain: []string{
				"probe",
				"address",
				"localhost:6379",
			},
			wantMissing: []string{
				"url",
				"pattern",
			},
		},
		{
			name: "log readiness uses pattern",
			cfg: &config.Config{
				Services: map[string]*config.Service{
					"worker": {
						Dir: "services/worker",
						Readiness: &config.Readiness{
							Type:    config.TypeLog,
							Pattern: "ready",
						},
					},
				},
			},
			service: &ServiceState{
				ID:     "worker",
				Name:   "worker",
				Status: StatusRunning,
			},
			wantContain: []string{
				"probe",
				"pattern",
				"ready",
			},
		},
		{
			name: "no readiness omits probe card",
			cfg: &config.Config{
				Services: map[string]*config.Service{
					"web": {Dir: "services/web"},
				},
			},
			service: &ServiceState{
				ID:     "web",
				Name:   "web",
				Status: StatusRunning,
			},
			wantMissing: []string{
				"probe",
			},
		},
		{
			name: "stopped service without pid omits pid card",
			cfg: &config.Config{
				Services: map[string]*config.Service{
					"web": {Dir: "services/web"},
				},
			},
			service: &ServiceState{
				ID:     "web",
				Name:   "web",
				Status: StatusStopped,
			},
			wantContain: []string{
				"status",
				"state",
				"stopped",
			},
			wantMissing: []string{
				"process",
				"pid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{theme: components.DefaultTheme(), cfg: tt.cfg}
			m.state.asideTab = AsideTabHealth
			m.state.now = now

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

func Test_FormatElapsed(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{
			name: "negative duration clamps to zero",
			in:   -5 * time.Second,
			want: "00:00",
		},
		{
			name: "seconds only",
			in:   9 * time.Second,
			want: "00:09",
		},
		{
			name: "minutes and seconds",
			in:   4*time.Minute + 9*time.Second,
			want: "04:09",
		},
		{
			name: "hours wraps",
			in:   2*time.Hour + 4*time.Minute + 9*time.Second,
			want: "02:04:09",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatElapsed(tt.in))
		})
	}
}

func Test_LookupServiceConfig(t *testing.T) {
	entry := &config.Service{Dir: "svc/api"}

	tests := []struct {
		name   string
		cfg    *config.Config
		lookup string
		want   *config.Service
	}{
		{
			name:   "nil config",
			cfg:    nil,
			lookup: "api",
			want:   nil,
		},
		{
			name:   "nil services map",
			cfg:    &config.Config{Services: nil},
			lookup: "api",
			want:   nil,
		},
		{
			name:   "missing service",
			cfg:    &config.Config{Services: map[string]*config.Service{}},
			lookup: "api",
			want:   nil,
		},
		{
			name:   "service found",
			cfg:    &config.Config{Services: map[string]*config.Service{"api": entry}},
			lookup: "api",
			want:   entry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{cfg: tt.cfg}
			assert.Equal(t, tt.want, m.lookupServiceConfig(tt.lookup))
		})
	}
}

func Test_AsideEnvTab(t *testing.T) {
	tests := []struct {
		name        string
		nilLoader   bool
		entries     []dotenv.Store
		wantContain []string
		wantMissing []string
	}{
		{
			name:      "nil dotenv loader shows availability message",
			nilLoader: true,
			wantContain: []string{
				"no environment variables available",
			},
		},
		{
			name:    "empty cache shows availability message",
			entries: nil,
			wantContain: []string{
				"no environment variables available",
			},
			wantMissing: []string{
				"APP_NAME",
			},
		},
		{
			name: "entries render under environment section",
			entries: []dotenv.Store{
				{Key: "APP_NAME", Value: "hub-api"},
				{Key: "JWT_SECRET", Value: "dev-wins"},
			},
			wantContain: []string{
				"environment",
				"APP_NAME", "hub-api",
				"JWT_SECRET", "dev-wins",
			},
			wantMissing: []string{
				"no environment variables available",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{theme: components.DefaultTheme()}

			if !tt.nilLoader {
				ctrl := gomock.NewController(t)
				mockEnv := dotenv.NewMockLoader(ctrl)
				mockEnv.EXPECT().Env("id-api").Return(tt.entries).AnyTimes()
				m.dotenv = mockEnv
			}

			m.state.asideTab = AsideTabEnv

			result := m.asideContent(&ServiceState{ID: "id-api", Name: "api", Status: StatusStopped}, 80)

			for _, want := range tt.wantContain {
				assert.Contains(t, result, want)
			}

			for _, miss := range tt.wantMissing {
				assert.NotContains(t, result, miss)
			}
		})
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
			assert.Equal(t, tt.want, nextAsideTab(tt.from))
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
			assert.Equal(t, tt.want, prevAsideTab(tt.from))
		})
	}
}

func Test_AsideScrollIndicator(t *testing.T) {
	tests := []struct {
		name      string
		lines     int
		gotoBot   bool
		wantEmpty bool
	}{
		{
			name:      "content fits inside viewport returns empty",
			lines:     2,
			wantEmpty: true,
		},
		{
			name:      "content overflows and at top returns percent",
			lines:     50,
			wantEmpty: false,
		},
		{
			name:      "content overflows and at bottom returns percent",
			lines:     50,
			gotoBot:   true,
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{theme: components.DefaultTheme()}
			m.ui.asideViewport.SetWidth(40)
			m.ui.asideViewport.SetHeight(5)
			m.ui.asideViewport.SetContent(strings.Repeat("line\n", tt.lines))

			if tt.gotoBot {
				m.ui.asideViewport.GotoBottom()
			}

			got := m.asideScrollIndicator()
			if tt.wantEmpty {
				assert.Empty(t, got)

				return
			}

			assert.Contains(t, got, "%")
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
			assert.Equal(t, tt.want, asideTabIndex(tt.tab))
		})
	}
}
