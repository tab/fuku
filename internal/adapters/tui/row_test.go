package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/adapters/terminal"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_GetStyledAndPaddedStatus(t *testing.T) {
	m := Model{}
	m.ui.layout = terminal.ComputeTableLayout(78-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)

	tests := []struct {
		name       string
		service    *model.Service
		isSelected bool
	}{
		{
			name:       "running status not selected",
			service:    &model.Service{Status: model.StatusRunning},
			isSelected: false,
		},
		{
			name:       "starting status not selected",
			service:    &model.Service{Status: model.StatusStarting},
			isSelected: false,
		},
		{
			name:       "failed status not selected",
			service:    &model.Service{Status: model.StatusFailed},
			isSelected: false,
		},
		{
			name:       "stopped status not selected",
			service:    &model.Service{Status: model.StatusStopped},
			isSelected: false,
		},
		{
			name:       "pending status not selected",
			service:    &model.Service{Status: model.StatusPending},
			isSelected: false,
		},
		{
			name:       "running status selected",
			service:    &model.Service{Status: model.StatusRunning},
			isSelected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.getStyledAndPaddedStatus(tt.service, tt.isSelected)

			assert.Contains(t, result, string(tt.service.Status))
		})
	}
}

func Test_GetStyledAndPaddedStatus_NoWatchIndicator(t *testing.T) {
	m := Model{}
	m.ui.layout = terminal.ComputeTableLayout(78-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)

	tests := []struct {
		name       string
		service    *model.Service
		isSelected bool
	}{
		{
			name:       "running with watching - indicator in indicator column not status",
			service:    &model.Service{Status: model.StatusRunning, Watching: true},
			isSelected: false,
		},
		{
			name:       "running without watching",
			service:    &model.Service{Status: model.StatusRunning, Watching: false},
			isSelected: false,
		},
		{
			name:       "running with watching selected",
			service:    &model.Service{Status: model.StatusRunning, Watching: true},
			isSelected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.getStyledAndPaddedStatus(tt.service, tt.isSelected)

			assert.Contains(t, result, string(tt.service.Status))
			assert.NotContains(t, result, terminal.IndicatorDot)
		})
	}
}

func Test_RenderServiceRow_Truncation(t *testing.T) {
	tests := []struct {
		name          string
		before        func() Model
		service       *model.Service
		wantNameInRow string
	}{
		{
			name: "short name no truncation",
			before: func() Model {
				m := Model{}
				m.ui.width = 108
				m.ui.layout = terminal.ComputeTableLayout(100-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(100)
				m.state.views = map[string]*serviceView{"id-svc": {Timeline: newTimeline(terminal.TimelineDefaultSlots)}}

				return m
			},
			service:       &model.Service{ID: "id-svc", Name: "api", Status: model.StatusRunning},
			wantNameInRow: "api",
		},
		{
			name: "long name truncated on narrow viewport",
			before: func() Model {
				m := Model{}
				m.ui.width = 86
				m.ui.layout = terminal.ComputeTableLayout(78-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(78)
				m.state.views = map[string]*serviceView{"id-svc": {Timeline: newTimeline(terminal.TimelineDefaultSlots)}}

				return m
			},
			service:       &model.Service{ID: "id-svc", Name: "action-confirmation-management-service", Status: model.StatusRunning},
			wantNameInRow: "action-confirmation-manageme…",
		},
		{
			name: "name fits exactly",
			before: func() Model {
				m := Model{}
				m.ui.width = 108
				m.ui.layout = terminal.ComputeTableLayout(100-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(100)
				m.state.views = map[string]*serviceView{"id-svc": {Timeline: newTimeline(terminal.TimelineDefaultSlots)}}

				return m
			},
			service:       &model.Service{ID: "id-svc", Name: "user-service", Status: model.StatusRunning},
			wantNameInRow: "user-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.renderServiceRow(tt.service, false)

			assert.Contains(t, result, tt.wantNameInRow)
		})
	}
}

func Test_RenderServiceRow_LongUptimeDoesNotWrap(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	now := time.Now()
	service := &model.Service{
		ID:     "id-api",
		Name:   "api",
		Status: model.StatusRunning,
		Process: model.Process{
			PID:       12345,
			CPU:       1.0,
			Memory:    64 * 1024 * 1024,
			StartedAt: now.Add(-100 * time.Hour),
		},
	}

	tests := []struct {
		name       string
		before     func() Model
		panelWidth int
		rowWidth   int
		uptime     string
	}{
		{
			name: "72-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 72
				m.ui.layout = terminal.ComputeTableLayout(72-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(72 - terminal.PanelInnerPadding)
				m.state.now = now
				m.state.views = map[string]*serviceView{"id-api": {Timeline: newTimeline(terminal.TimelineDefaultSlots)}}

				return m
			},
			panelWidth: 72,
			rowWidth:   72 - terminal.PanelInnerPadding,
			uptime:     "100:0…",
		},
		{
			name: "104-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 104
				m.ui.layout = terminal.ComputeTableLayout(104-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(104 - terminal.PanelInnerPadding)
				m.state.now = now
				m.state.views = map[string]*serviceView{"id-api": {Timeline: newTimeline(terminal.TimelineDefaultSlots)}}

				return m
			},
			panelWidth: 104,
			rowWidth:   104 - terminal.PanelInnerPadding,
			uptime:     "100:00:00",
		},
		{
			name: "120-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 120
				m.ui.layout = terminal.ComputeTableLayout(120-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(120 - terminal.PanelInnerPadding)
				m.state.now = now
				m.state.views = map[string]*serviceView{"id-api": {Timeline: newTimeline(terminal.TimelineDefaultSlots)}}

				return m
			},
			panelWidth: 120,
			rowWidth:   120 - terminal.PanelInnerPadding,
			uptime:     "100:00:00",
		},
		{
			name: "200-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 200
				m.ui.layout = terminal.ComputeTableLayout(200-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(200 - terminal.PanelInnerPadding)
				m.state.now = now
				m.state.views = map[string]*serviceView{"id-api": {Timeline: newTimeline(terminal.TimelineDefaultSlots)}}

				return m
			},
			panelWidth: 200,
			rowWidth:   200 - terminal.PanelInnerPadding,
			uptime:     "100:00:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			row := m.renderServiceRow(service, false)

			assert.NotContains(t, row, "\n", "row wraps with 9-char uptime")
			assert.Equal(t, tt.rowWidth, lipgloss.Width(row), "row width must equal rowWidth at %d cols", tt.panelWidth)
			assert.Contains(t, row, tt.uptime, "the metric column shows the uptime its width fits at %d cols", tt.panelWidth)
		})
	}
}

func Test_RenderServiceRow_LongSharedPrefixNamesDistinguishableOnWideTerminal(t *testing.T) {
	nameA := "very-long-service-name-with-shared-prefix-section-alpha-tail"
	nameB := "very-long-service-name-with-shared-prefix-section-bravo-tail"

	panelWidth := 200
	rowWidth := panelWidth - terminal.PanelInnerPadding

	m := Model{}
	m.ui.width = panelWidth
	m.ui.servicesViewport.SetWidth(rowWidth)
	m.theme = terminal.NewTheme(terminal.AppearanceDark)
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-a": {ID: "id-a", Name: nameA, Status: model.StatusRunning},
		"id-b": {ID: "id-b", Name: nameB, Status: model.StatusRunning},
	}}
	m.state.views = map[string]*serviceView{
		"id-a": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
		"id-b": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
	}

	m = m.recomputeLayout()

	rowA := m.renderServiceRow(m.snapshot.Services["id-a"], false)
	rowB := m.renderServiceRow(m.snapshot.Services["id-b"], false)

	assert.Contains(t, rowA, nameA, "full name should render at wide terminal when name exceeds 48 cells")
	assert.Contains(t, rowB, nameB, "full name should render at wide terminal when name exceeds 48 cells")
	assert.NotEqual(t, rowA, rowB, "shared-prefix names beyond 48 cells must remain distinguishable on wide terminals")
}

func Test_RenderServiceRow_UnicodeNameTimelineSurvivesNarrow(t *testing.T) {
	rowWidth := 72 - terminal.PanelInnerPadding
	service := &model.Service{ID: "id-svc", Name: "сервис-апи", Status: model.StatusRunning}

	m := Model{}
	m.ui.width = 72
	m.ui.servicesViewport.SetWidth(rowWidth)
	m.theme = terminal.NewTheme(terminal.AppearanceDark)
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-svc": service}}
	m.state.views = map[string]*serviceView{
		"id-svc": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
	}
	m = m.recomputeLayout()
	require.Positive(t, m.ui.layout.TimelineWidth, "10-cell Cyrillic name should pick short bucket and keep timeline visible")

	row := m.renderServiceRow(service, false)

	assert.Contains(t, row, "сервис-апи")
	assert.Equal(t, rowWidth, lipgloss.Width(row), "row width must equal rowWidth")
}

func Test_RenderServiceRow_SharedPrefixNamesDistinguishable(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	svcA := &model.Service{
		ID:     "id-a",
		Name:   "action-confirmation-management-service",
		Status: model.StatusRunning,
	}
	svcB := &model.Service{
		ID:     "id-b",
		Name:   "action-confirmation-metrics-service",
		Status: model.StatusRunning,
	}

	tests := []struct {
		name       string
		before     func() Model
		panelWidth int
	}{
		{
			name: "72-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 72
				m.ui.layout = terminal.ComputeTableLayout(72-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(72 - terminal.PanelInnerPadding)
				m.state.views = map[string]*serviceView{
					"id-a": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
					"id-b": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
				}

				return m
			},
			panelWidth: 72,
		},
		{
			name: "84-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 84
				m.ui.layout = terminal.ComputeTableLayout(84-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(84 - terminal.PanelInnerPadding)
				m.state.views = map[string]*serviceView{
					"id-a": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
					"id-b": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
				}

				return m
			},
			panelWidth: 84,
		},
		{
			name: "90-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 90
				m.ui.layout = terminal.ComputeTableLayout(90-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(90 - terminal.PanelInnerPadding)
				m.state.views = map[string]*serviceView{
					"id-a": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
					"id-b": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
				}

				return m
			},
			panelWidth: 90,
		},
		{
			name: "104-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 104
				m.ui.layout = terminal.ComputeTableLayout(104-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(104 - terminal.PanelInnerPadding)
				m.state.views = map[string]*serviceView{
					"id-a": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
					"id-b": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
				}

				return m
			},
			panelWidth: 104,
		},
		{
			name: "120-col terminal",
			before: func() Model {
				m := Model{theme: theme}
				m.ui.width = 120
				m.ui.layout = terminal.ComputeTableLayout(120-terminal.PanelInnerPadding-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
				m.ui.servicesViewport.SetWidth(120 - terminal.PanelInnerPadding)
				m.state.views = map[string]*serviceView{
					"id-a": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
					"id-b": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
				}

				return m
			},
			panelWidth: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			rowA := m.renderServiceRow(svcA, false)
			rowB := m.renderServiceRow(svcB, false)

			assert.NotEqual(t, rowA, rowB, "shared-prefix names must produce different rows at %d columns", tt.panelWidth)
		})
	}
}

func Test_RenderServiceRow_ColumnAlignment(t *testing.T) {
	m := Model{}
	m.ui.width = 120
	m.ui.layout = terminal.ComputeTableLayout(112-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
	m.ui.servicesViewport.SetWidth(112)

	service1 := &model.Service{Name: "api", Status: model.StatusRunning}
	service2 := &model.Service{Name: "user-management-service", Status: model.StatusStarting}

	row1 := m.renderServiceRow(service1, false)
	row2 := m.renderServiceRow(service2, false)

	assert.Contains(t, row1, "api")
	assert.Contains(t, row1, "running")
	assert.Contains(t, row2, "user-management-service")
	assert.Contains(t, row2, "starting")
}

func Test_RenderServiceRow_SelectedIndicator(t *testing.T) {
	m := Model{}
	m.ui.width = 100
	m.ui.layout = terminal.ComputeTableLayout(92-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
	m.ui.servicesViewport.SetWidth(92)

	service := &model.Service{Name: "api", Status: model.StatusRunning}

	unselectedRow := m.renderServiceRow(service, false)
	selectedRow := m.renderServiceRow(service, true)

	assert.Contains(t, unselectedRow, "  ")
	assert.Contains(t, selectedRow, terminal.IndicatorSelected+" ")
}

func Test_RenderServiceRow_SelectedBackgroundCoversFullWidth(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	m := Model{theme: theme}
	m.ui.width = 120
	m.ui.layout = terminal.ComputeTableLayout(112-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
	m.ui.servicesViewport.SetWidth(112)

	tl := newTimeline(terminal.TimelineDefaultSlots)
	for range 10 {
		tl.Append(TimelineSlotRunning)
	}

	m.state.views = map[string]*serviceView{"id-api": {Timeline: tl}}

	service := &model.Service{
		ID:     "id-api",
		Name:   "api",
		Status: model.StatusRunning,
		Process: model.Process{
			PID:    12345,
			CPU:    1.5,
			Memory: 64 * 1024 * 1024,
		},
	}

	row := m.renderServiceRow(service, true)

	statusIndex := strings.Index(row, "running")
	assert.NotEqual(t, -1, statusIndex, "row must contain 'running' status")

	metricsIndex := strings.Index(row, "12345")
	assert.NotEqual(t, -1, metricsIndex, "row must contain PID metrics")
	assert.Greater(t, metricsIndex, statusIndex, "PID metrics must appear after status")

	segment := row[statusIndex:metricsIndex]
	assert.NotContains(t, segment, "\x1b[m", "selection background must not reset between status and metrics")
}

func Test_GetServiceDetails_WithError(t *testing.T) {
	m := Model{}

	tests := []struct {
		name     string
		service  *model.Service
		expected string
	}{
		{
			name: "port already in use",
			service: &model.Service{
				Name:   "api",
				Status: model.StatusFailed,
				Error:  contracts.ErrPortAlreadyInUse.Error(),
			},
			expected: "port already in use",
		},
		{
			name: "max retries exceeded",
			service: &model.Service{
				Name:   "api",
				Status: model.StatusFailed,
				Error:  contracts.ErrMaxRetriesExceeded.Error(),
			},
			expected: "max retries exceeded",
		},
		{
			name: "readiness timeout",
			service: &model.Service{
				Name:   "api",
				Status: model.StatusFailed,
				Error:  contracts.ErrReadinessTimeout.Error(),
			},
			expected: "readiness timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.getServiceDetails(tt.service, true)

			assert.Contains(t, result, tt.expected)
		})
	}
}

func Test_GetServiceDetails_WithMetrics(t *testing.T) {
	m := Model{}
	m.ui.layout = terminal.ComputeTableLayout(78-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)

	service := &model.Service{
		Name:   "api",
		Status: model.StatusRunning,
		Process: model.Process{
			PID:    12345,
			CPU:    5.5,
			Memory: 128 * 1024 * 1024,
		},
	}

	result := m.getServiceDetails(service, false)

	assert.Contains(t, result, "5.5%")
	assert.Contains(t, result, "128MB")
	assert.Contains(t, result, "12345")
}

func Test_GetServiceDetails_NoMetricsWhenStopped(t *testing.T) {
	m := Model{}
	m.ui.layout = terminal.ComputeTableLayout(78-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)

	service := &model.Service{
		Name:   "api",
		Status: model.StatusStopped,
	}

	result := m.getServiceDetails(service, false)

	assert.NotContains(t, result, "%")
	assert.NotContains(t, result, "MB")
}

func Test_RenderTier_Spacing(t *testing.T) {
	m := Model{}
	m.ui.width = 100
	m.ui.layout = terminal.ComputeTableLayout(92-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
	m.ui.servicesViewport.SetWidth(92)

	api := &model.Service{ID: "api", Name: "api", Status: model.StatusRunning}
	db := &model.Service{ID: "db", Name: "db", Status: model.StatusRunning}
	tier := &model.Tier{Name: "platform", Services: []*model.Service{api, db}}
	currentIdx := 0

	result := m.renderTier(tier, &currentIdx)

	assert.Contains(t, result, "platform")
	assert.Contains(t, result, "api")
	assert.Contains(t, result, "db")
}

func Test_RenderTier_ServiceCount(t *testing.T) {
	m := Model{}
	m.ui.width = 100
	m.ui.layout = terminal.ComputeTableLayout(92-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
	m.ui.servicesViewport.SetWidth(92)

	api := &model.Service{ID: "api", Name: "api", Status: model.StatusRunning}
	db := &model.Service{ID: "db", Name: "db", Status: model.StatusRunning}
	web := &model.Service{ID: "web", Name: "web", Status: model.StatusRunning}
	tier := &model.Tier{Name: "platform", Services: []*model.Service{api, db, web}}
	currentIdx := 0

	result := m.renderTier(tier, &currentIdx)

	assert.Equal(t, 3, currentIdx)
	assert.Contains(t, result, "api")
	assert.Contains(t, result, "db")
	assert.Contains(t, result, "web")
}

func Test_GetServiceIndicator_DefaultNotSelected(t *testing.T) {
	m := Model{}
	service := &model.Service{Name: "api", Status: model.StatusStopped}

	result := m.getServiceIndicator(service, false)

	assert.Equal(t, " ", result)
}

func Test_GetServiceIndicator_DefaultSelected(t *testing.T) {
	m := Model{}
	service := &model.Service{Name: "api", Status: model.StatusStopped}

	result := m.getServiceIndicator(service, true)

	assert.Equal(t, terminal.IndicatorSelected, result)
}

func Test_GetServiceIndicator_NonTransitionalStatus(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	m := Model{}
	m.theme = theme

	tests := []struct {
		name       string
		service    *model.Service
		isSelected bool
		want       string
	}{
		{
			name:       "running not selected",
			service:    &model.Service{Name: "api", Status: model.StatusRunning, Watching: false},
			isSelected: false,
			want:       " ",
		},
		{
			name:       "running selected",
			service:    &model.Service{Name: "api", Status: model.StatusRunning, Watching: false},
			isSelected: true,
			want:       terminal.IndicatorSelected,
		},
		{
			name:       "watching running not selected shows watch indicator",
			service:    &model.Service{Name: "api", Status: model.StatusRunning, Watching: true},
			isSelected: false,
			want:       theme.IndicatorDotStyle.Render(terminal.IndicatorDot),
		},
		{
			name:       "watching running selected shows watch indicator unstyled",
			service:    &model.Service{Name: "api", Status: model.StatusRunning, Watching: true},
			isSelected: true,
			want:       terminal.IndicatorDot,
		},
		{
			name:       "watching stopped does not show watch indicator",
			service:    &model.Service{Name: "api", Status: model.StatusStopped, Watching: true},
			isSelected: false,
			want:       " ",
		},
		{
			name:       "stopped not selected",
			service:    &model.Service{Name: "api", Status: model.StatusStopped, Watching: false},
			isSelected: false,
			want:       " ",
		},
		{
			name:       "failed not selected",
			service:    &model.Service{Name: "api", Status: model.StatusFailed, Watching: false},
			isSelected: false,
			want:       " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.getServiceIndicator(tt.service, tt.isSelected)

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_GetWatchIndicator(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	m := Model{}
	m.theme = theme

	tests := []struct {
		name       string
		isSelected bool
		want       string
	}{
		{
			name:       "not selected returns styled indicator",
			isSelected: false,
			want:       theme.IndicatorDotStyle.Render(terminal.IndicatorDot),
		},
		{
			name:       "selected returns unstyled indicator",
			isSelected: true,
			want:       terminal.IndicatorDot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.getWatchIndicator(tt.isSelected)

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_GetServiceIndicator_ViewNotBuilt(t *testing.T) {
	m := Model{}
	m.state.views = map[string]*serviceView{}

	tests := []struct {
		name       string
		service    *model.Service
		isSelected bool
		want       string
	}{
		{
			name:       "starting status without a view not selected",
			service:    &model.Service{ID: "api", Name: "api", Status: model.StatusStarting},
			isSelected: false,
			want:       " ",
		},
		{
			name:       "stopping status without a view selected",
			service:    &model.Service{ID: "api", Name: "api", Status: model.StatusStopping},
			isSelected: true,
			want:       terminal.IndicatorSelected,
		},
		{
			name:       "restarting status without a view not selected",
			service:    &model.Service{ID: "api", Name: "api", Status: model.StatusRestarting},
			isSelected: false,
			want:       " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.getServiceIndicator(tt.service, tt.isSelected)

			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_GetServiceIndicator_BlinkIndicatorNotSelected(t *testing.T) {
	m := Model{}
	blink := terminal.NewBlink()
	m.state.views = map[string]*serviceView{"api": {Blink: blink}}

	tests := []struct {
		name    string
		service *model.Service
	}{
		{
			name:    "starting status with blink",
			service: &model.Service{ID: "api", Name: "api", Status: model.StatusStarting},
		},
		{
			name:    "stopping status with blink",
			service: &model.Service{ID: "api", Name: "api", Status: model.StatusStopping},
		},
		{
			name:    "restarting status with blink",
			service: &model.Service{ID: "api", Name: "api", Status: model.StatusRestarting},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.getServiceIndicator(tt.service, false)

			assert.NotEqual(t, " ", result)
			assert.NotEmpty(t, result)
		})
	}
}

func Test_GetServiceIndicator_BlinkIndicatorSelected(t *testing.T) {
	m := Model{}
	blink := terminal.NewBlink()
	m.state.views = map[string]*serviceView{"api": {Blink: blink}}

	tests := []struct {
		name    string
		service *model.Service
	}{
		{
			name:    "starting status selected",
			service: &model.Service{ID: "api", Name: "api", Status: model.StatusStarting},
		},
		{
			name:    "stopping status selected",
			service: &model.Service{ID: "api", Name: "api", Status: model.StatusStopping},
		},
		{
			name:    "restarting status selected",
			service: &model.Service{ID: "api", Name: "api", Status: model.StatusRestarting},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.getServiceIndicator(tt.service, true)

			assert.Equal(t, blink.Frame(), result)
		})
	}
}

func Test_RenderTimeline_AllSlotTypes(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	m := Model{theme: theme}
	m.ui.layout = terminal.TableLayout{TimelineWidth: 5}

	tl := newTimeline(5)
	tl.Append(TimelineSlotRunning)
	tl.Append(TimelineSlotStarting)
	tl.Append(TimelineSlotFailed)
	tl.Append(TimelineSlotStopped)
	tl.Append(TimelineSlotEmpty)

	m.state.views = map[string]*serviceView{"id-svc": {Timeline: tl}}
	service := &model.Service{ID: "id-svc"}

	result := m.renderTimeline(service, false)

	assert.Contains(t, result, theme.TimelineRunningStyle.Render(terminal.TimelineBlock))
	assert.Contains(t, result, theme.TimelineStartingStyle.Render(terminal.TimelineBlock))
	assert.Contains(t, result, theme.TimelineFailedStyle.Render(terminal.TimelineBlock))
	assert.Contains(t, result, theme.TimelineStoppedStyle.Render(terminal.TimelineBlock))
	assert.Contains(t, result, theme.TimelineEmptyStyle.Render(terminal.TimelineBlock))
}

func Test_RenderTimeline_ZeroWidthReturnsEmpty(t *testing.T) {
	m := Model{}
	m.ui.layout = terminal.TableLayout{TimelineWidth: 0}

	m.state.views = map[string]*serviceView{"id-svc": {Timeline: newTimeline(20)}}
	service := &model.Service{ID: "id-svc"}

	result := m.renderTimeline(service, false)

	assert.Empty(t, result)
}

func Test_RenderTimeline_ViewNotBuiltReturnsPadding(t *testing.T) {
	m := Model{}
	m.ui.layout = terminal.TableLayout{TimelineWidth: 10}

	m.state.views = map[string]*serviceView{}
	service := &model.Service{ID: "id-svc"}

	result := m.renderTimeline(service, false)

	assert.Equal(t, strings.Repeat(" ", 10), result)
}

func Test_RenderTimeline_ReducedWidthShowsRecentSlots(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	m := Model{theme: theme}
	m.ui.layout = terminal.TableLayout{TimelineWidth: 5}

	tl := newTimeline(10)
	for range 10 {
		tl.Append(TimelineSlotRunning)
	}

	tl.Append(TimelineSlotFailed)

	m.state.views = map[string]*serviceView{"id-svc": {Timeline: tl}}
	service := &model.Service{ID: "id-svc"}

	result := m.renderTimeline(service, false)

	assert.Equal(t, 5, lipgloss.Width(result))
	assert.Contains(t, result, theme.TimelineFailedStyle.Render(terminal.TimelineBlock))
}

func Test_RenderTimeline_ReducedWidthPartiallyFilled(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	m := Model{theme: theme}
	m.ui.layout = terminal.TableLayout{TimelineWidth: 5}

	tl := newTimeline(20)
	tl.Append(TimelineSlotRunning)
	tl.Append(TimelineSlotFailed)

	m.state.views = map[string]*serviceView{"id-svc": {Timeline: tl}}
	service := &model.Service{ID: "id-svc"}

	result := m.renderTimeline(service, false)

	assert.Equal(t, 5, lipgloss.Width(result))
	assert.Contains(t, result, theme.TimelineRunningStyle.Render(terminal.TimelineBlock))
	assert.Contains(t, result, theme.TimelineFailedStyle.Render(terminal.TimelineBlock))
}

func Test_RenderTimeline_SelectedUsesSelectionAwareStyles(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	m := Model{theme: theme}
	m.ui.layout = terminal.TableLayout{TimelineWidth: 5}

	tl := newTimeline(5)
	tl.Append(TimelineSlotRunning)
	tl.Append(TimelineSlotFailed)

	m.state.views = map[string]*serviceView{"id-svc": {Timeline: tl}}
	service := &model.Service{ID: "id-svc"}

	result := m.renderTimeline(service, true)

	runningBlock := theme.TimelineSelectedRunningStyle.Render(terminal.TimelineBlock)
	failedBlock := theme.TimelineSelectedFailedStyle.Render(terminal.TimelineBlock)
	emptyBlock := theme.TimelineSelectedEmptyStyle.Render(terminal.TimelineBlock)

	expected := runningBlock + failedBlock + strings.Repeat(emptyBlock, 3)
	assert.Equal(t, expected, result)
}

func Test_RenderServiceRow_WithTimeline(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	m := Model{theme: theme}
	m.ui.width = 120
	m.ui.layout = terminal.ComputeTableLayout(112-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
	m.ui.servicesViewport.SetWidth(112)

	tl := newTimeline(terminal.TimelineDefaultSlots)
	tl.Append(TimelineSlotRunning)

	m.state.views = map[string]*serviceView{"id-api": {Timeline: tl}}
	service := &model.Service{ID: "id-api", Name: "api", Status: model.StatusRunning}

	result := m.renderServiceRow(service, false)

	assert.Contains(t, result, "api")
	assert.Contains(t, result, "running")
	assert.Contains(t, result, terminal.TimelineBlock)
}

func Test_RenderServiceRow_ErrorRowStillShowsTimeline(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	m := Model{theme: theme}
	m.ui.width = 120
	m.ui.layout = terminal.ComputeTableLayout(112-terminal.RowHorizontalPadding, terminal.ServiceNameWidthLong, terminal.MetricFullColumnCount)
	m.ui.servicesViewport.SetWidth(112)

	tl := newTimeline(terminal.TimelineDefaultSlots)
	tl.Append(TimelineSlotFailed)

	m.state.views = map[string]*serviceView{"id-api": {Timeline: tl}}

	service := &model.Service{
		ID:     "id-api",
		Name:   "api",
		Status: model.StatusFailed,
		Error:  contracts.ErrPortAlreadyInUse.Error(),
	}

	result := m.renderServiceRow(service, false)

	assert.Contains(t, result, "api")
	assert.Contains(t, result, "failed")
	assert.Contains(t, result, terminal.TimelineBlock)
	assert.Contains(t, result, "port already in use")
}
