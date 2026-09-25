package tui

import (
	"image/color"
	"log/slog"
	"os"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/resources"
	"fuku/internal/adapters/terminal"
	"fuku/internal/app/services"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_HandleKeyPress_ForceQuitWithCtrlC(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, log: log}
	m.state.shuttingDown = false
	m.ui.servicesKeys = defaultKeyMap()

	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}

	result, cmd := m.handleKeyPress(msg)

	assert.False(t, result.loader.Active)
	assert.NotNil(t, cmd)
}

func Test_HandleKeyPress_IgnoresQuitWhileShuttingDown(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader}
	m.state.shuttingDown = true
	m.ui.servicesKeys = defaultKeyMap()

	msg := tea.KeyPressMsg{Code: 'q', Text: "q"}

	result, cmd := m.handleKeyPress(msg)

	assert.True(t, result.state.shuttingDown)
	assert.Nil(t, cmd)
}

func Test_HandleKeyPress_IgnoresOtherKeysWhileShuttingDown(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader}
	m.state.shuttingDown = true
	m.ui.servicesKeys = defaultKeyMap()

	msg := tea.KeyPressMsg{Code: 'j', Text: "j"}

	result, cmd := m.handleKeyPress(msg)

	assert.True(t, result.state.shuttingDown)
	assert.Nil(t, cmd)
}

func Test_HandleKeyPress_QuitAsksTheCoreToStopAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockControl := NewMockControl(ctrl)
	mockControl.EXPECT().StopAll().Return(nil)

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, control: mockControl}
	m.state.shuttingDown = false
	m.ui.servicesKeys = defaultKeyMap()

	msg := tea.KeyPressMsg{Code: 'q', Text: "q"}

	result, cmd := m.handleKeyPress(msg)

	assert.Equal(t, stopAllMsg{}, cmd())
	assert.False(t, result.state.shuttingDown)
	assert.False(t, result.loader.Active)
}

func Test_Update_KeyMsg_CtrlCForceQuitsDuringShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(&model.Snapshot{}))

	log := slog.New(slog.DiscardHandler)

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, registry: mockRegistry, log: log}
	m.state.shuttingDown = true
	m.ui.servicesKeys = defaultKeyMap()

	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}

	_, cmd := m.Update(msg)

	assert.NotNil(t, cmd)
}

func Test_Update_KeyMsg_IgnoreQuitDuringShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(&model.Snapshot{}))

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, registry: mockRegistry}
	m.state.shuttingDown = true
	m.ui.servicesKeys = defaultKeyMap()

	msg := tea.KeyPressMsg{Code: 'q', Text: "q"}

	teaModel, cmd := m.Update(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.Nil(t, cmd)
}

func Test_Update_EventMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)
	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(&model.Snapshot{}))

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, registry: mockRegistry}
	m.state.shuttingDown = false

	event := contracts.Message{Type: contracts.EventSignalReceived, Data: contracts.SignalReceived{Name: "SIGINT"}}
	msg := EventMsg(event)

	teaModel, cmd := m.Update(msg)
	result := teaModel.(Model)

	assert.True(t, result.state.shuttingDown)
	assert.Nil(t, cmd)
}

func Test_Update_ClearsTheSnapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	log := slog.New(slog.DiscardHandler)

	resolved := &model.Snapshot{Phase: model.PhaseStartup, Resolved: true}
	changed := contracts.Message{Type: contracts.EventSnapshotChanged, Data: contracts.SnapshotChanged{}}

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(resolved))

	m := Model{loader: NewLoader(), registry: mockRegistry, log: log}

	teaModel, _ := m.Update(EventMsg(changed))
	result := teaModel.(Model)

	assert.True(t, result.state.resolved)
	assert.Nil(t, result.snapshot)
}

func Test_Update_ActionAnswers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	log := slog.New(slog.DiscardHandler)

	empty := &model.Snapshot{}
	starting := services.Admission{Service: model.Service{ID: "id-api", Name: "api"}, Action: contracts.ActionStart, Status: model.StatusStarting}

	tests := []struct {
		name         string
		before       func() Model
		msg          tea.Msg
		expectLoader string
	}{
		{
			name: "an admission starts the service loader",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(empty))

				return Model{loader: NewLoader(), registry: mockRegistry, log: log}
			},
			msg:          admissionMsg{name: "api", admission: starting},
			expectLoader: "id-api",
		},
		{
			name: "an accepted stop all starts the shutdown loader",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(empty))

				return Model{loader: NewLoader(), registry: mockRegistry, log: log}
			},
			msg:          stopAllMsg{},
			expectLoader: loaderKeyShutdown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			teaModel, _ := m.Update(tt.msg)
			result := teaModel.(Model)

			assert.True(t, result.loader.Has(tt.expectLoader))
		})
	}
}

func Test_Update_ViewMessages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	log := slog.New(slog.DiscardHandler)
	theme := terminal.NewTheme(terminal.AppearanceDark)

	snapshot := &model.Snapshot{}
	firstFrame := terminal.SpinnerStyle.Render(spinner.MiniDot.Frames[0])
	secondFrame := terminal.SpinnerStyle.Render(spinner.MiniDot.Frames[1])

	fresh := func() Model {
		mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))

		return NewModel(t.Context(), ModelParams{Registry: mockRegistry, Theme: theme, Logger: log})
	}

	tests := []struct {
		name               string
		before             func() Model
		msg                tea.Msg
		expectedAppearance terminal.Appearance
		expectedSpinner    string
		expectedCPU        float64
		expectedMEM        float64
		expectedCmd        bool
	}{
		{
			name:               "a light background switches to the light theme",
			before:             fresh,
			msg:                tea.BackgroundColorMsg{Color: color.White},
			expectedAppearance: terminal.AppearanceLight,
			expectedSpinner:    firstFrame,
		},
		{
			name:               "a dark background keeps the dark theme",
			before:             fresh,
			msg:                tea.BackgroundColorMsg{Color: color.Black},
			expectedAppearance: terminal.AppearanceDark,
			expectedSpinner:    firstFrame,
		},
		{
			name:               "a spinner tick advances the loader and schedules the next tick",
			before:             fresh,
			msg:                spinner.TickMsg{},
			expectedAppearance: terminal.AppearanceDark,
			expectedSpinner:    secondFrame,
			expectedCmd:        true,
		},
		{
			name:               "sampled app stats are kept for the status bar",
			before:             fresh,
			msg:                appStatsMsg{cpu: 1.5, mem: 64},
			expectedAppearance: terminal.AppearanceDark,
			expectedSpinner:    firstFrame,
			expectedCPU:        1.5,
			expectedMEM:        64,
		},
		{
			name:               "an unhandled message changes nothing",
			before:             fresh,
			msg:                tea.FocusMsg{},
			expectedAppearance: terminal.AppearanceDark,
			expectedSpinner:    firstFrame,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			teaModel, cmd := m.Update(tt.msg)
			result := teaModel.(Model)

			assert.Equal(t, tt.expectedAppearance, result.theme.Appearance)
			assert.Equal(t, tt.expectedSpinner, result.loader.Model.View())
			assert.InDelta(t, tt.expectedCPU, result.state.appCPU, 0)
			assert.InDelta(t, tt.expectedMEM, result.state.appMEM, 0)
			assert.Equal(t, tt.expectedCmd, cmd != nil)
		})
	}
}

func Test_HandleKeyPress_SlashEntersFilterMode(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader}
	m.state.shuttingDown = false
	m.ui.servicesKeys = defaultKeyMap()

	msg := tea.KeyPressMsg{Code: '/', Text: "/"}

	result, cmd := m.handleKeyPress(msg)

	assert.True(t, result.state.filterActive)
	assert.Nil(t, cmd)
}

func Test_HandleKeyPress_FilterActiveRoutesToFilterInput(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader}
	m.state.filterActive = true
	m.state.filterQuery = ""
	m.state.serviceIDs = []string{"id-api"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"]}},
	}
	m.ui.servicesKeys = defaultKeyMap()

	msg := tea.KeyPressMsg{Code: 'a', Text: "a"}

	result, _ := m.handleKeyPress(msg)

	assert.Equal(t, "a", result.state.filterQuery)
	assert.True(t, result.state.filterActive)
}

func Test_HandleKeyPress_EscClearsFilterAfterEnter(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader}
	m.state.filterActive = false
	m.state.filterQuery = "web"
	m.state.filteredIDs = []string{"id-web"}
	m.state.serviceIDs = []string{"id-api", "id-web"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"]}},
	}
	m.ui.servicesKeys = defaultKeyMap()

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}

	result, _ := m.handleKeyPress(msg)

	assert.Empty(t, result.state.filterQuery)
	assert.False(t, result.state.filterActive)
	assert.Nil(t, result.state.filteredIDs)
}

func Test_HandleKeyPress_EscClearsSeparatorOnlyQuery(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader}
	m.state.filterActive = false
	m.state.filterQuery = "---"
	m.state.filteredIDs = []string{"id-api"}
	m.state.serviceIDs = []string{"id-api", "id-web"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"]}},
	}
	m.ui.servicesKeys = defaultKeyMap()

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}

	result, _ := m.handleKeyPress(msg)

	assert.Empty(t, result.state.filterQuery)
	assert.False(t, result.state.filterActive)
	assert.Nil(t, result.state.filteredIDs)
}

func Test_HandleUpKey_WithFilter(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"], m.snapshot.Services["id-db"]}},
	}
	m.state.filterQuery = "b"
	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.selected = 1

	result, _ := m.handleUpKey(tea.KeyPressMsg{Code: tea.KeyUp})

	assert.Equal(t, 0, result.state.selected)
}

func Test_HandleUpKey_WithFilter_AtTop(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.state.filterQuery = "api"
	m.state.filteredIDs = []string{"id-api"}
	m.state.selected = 0

	result, _ := m.handleUpKey(tea.KeyPressMsg{Code: tea.KeyUp})

	assert.Equal(t, 0, result.state.selected)
}

func Test_HandleDownKey_WithFilter(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"], m.snapshot.Services["id-db"]}},
	}
	m.state.filterQuery = "b"
	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.selected = 0

	result, _ := m.handleDownKey(tea.KeyPressMsg{Code: tea.KeyDown})

	assert.Equal(t, 1, result.state.selected)
}

func Test_HandleDownKey_WithFilter_AtBottom(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
		"id-db":  {ID: "id-db", Name: "db"},
	}}
	m.state.filterQuery = "api"
	m.state.filteredIDs = []string{"id-api"}
	m.state.selected = 0

	result, _ := m.handleDownKey(tea.KeyPressMsg{Code: tea.KeyDown})

	assert.Equal(t, 0, result.state.selected)
}

func Test_HandleDownKey_WithFilter_ZeroMatches(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
	}}
	m.state.filterQuery = "xyz"
	m.state.filteredIDs = []string{}
	m.state.selected = 0

	result, _ := m.handleDownKey(tea.KeyPressMsg{Code: tea.KeyDown})

	assert.Equal(t, 0, result.state.selected)
}

func Test_HandleStopKey_WithFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockControl := NewMockControl(ctrl)

	stopping := services.Admission{Service: model.Service{ID: "id-web", Name: "web"}, Action: contracts.ActionStop, Status: model.StatusStopping}

	mockControl.EXPECT().Toggle("id-web").Return(stopping, nil)

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, control: mockControl}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api", Status: model.StatusRunning},
		"id-web": {ID: "id-web", Name: "web", Status: model.StatusRunning},
		"id-db":  {ID: "id-db", Name: "db", Status: model.StatusRunning},
	}}
	m.state.filterQuery = "web"
	m.state.filteredIDs = []string{"id-web"}
	m.state.selected = 0

	_, cmd := m.handleStopKey()

	assert.Equal(t, admissionMsg{name: "web", admission: stopping}, cmd())
}

func Test_HandleStopKey_ZeroMatches_IsNoop(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader}
	m.state.serviceIDs = []string{"id-api"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api", Status: model.StatusRunning},
	}}
	m.state.filterQuery = "xyz"
	m.state.filteredIDs = []string{}
	m.state.selected = 0

	result, cmd := m.handleStopKey()

	assert.False(t, result.loader.Active)
	assert.Nil(t, cmd)
}

func Test_HandleRestartKey_WithFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockControl := NewMockControl(ctrl)

	restarting := services.Admission{Service: model.Service{ID: "id-db", Name: "db"}, Action: contracts.ActionRestart, Status: model.StatusRestarting}

	mockControl.EXPECT().Restart("id-db").Return(restarting, nil)

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, control: mockControl}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api", Status: model.StatusRunning},
		"id-web": {ID: "id-web", Name: "web", Status: model.StatusRunning},
		"id-db":  {ID: "id-db", Name: "db", Status: model.StatusRunning},
	}}
	m.state.filterQuery = "db"
	m.state.filteredIDs = []string{"id-db"}
	m.state.selected = 0

	_, cmd := m.handleRestartKey()

	assert.Equal(t, admissionMsg{name: "db", admission: restarting}, cmd())
}

func Test_HandleRestartKey_ZeroMatches_IsNoop(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader}
	m.state.serviceIDs = []string{"id-api"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api", Status: model.StatusRunning},
	}}
	m.state.filterQuery = "xyz"
	m.state.filteredIDs = []string{}
	m.state.selected = 0

	result, cmd := m.handleRestartKey()

	assert.False(t, result.loader.Active)
	assert.Nil(t, cmd)
}

func Test_CalculateScrollOffset_WithFilter(t *testing.T) {
	m := Model{}
	m.state.serviceIDs = []string{"id-api", "id-web", "id-db", "id-cache"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api":   {ID: "id-api", Name: "api"},
		"id-web":   {ID: "id-web", Name: "web"},
		"id-db":    {ID: "id-db", Name: "db"},
		"id-cache": {ID: "id-cache", Name: "cache"},
	}}
	m.snapshot.Tiers = []*model.Tier{
		{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"], m.snapshot.Services["id-web"]}},
		{Name: "tier2", Services: []*model.Service{m.snapshot.Services["id-db"], m.snapshot.Services["id-cache"]}},
	}
	m.state.filterQuery = "b"
	m.state.filteredIDs = []string{"id-web", "id-db"}
	m.state.selected = 1
	m.ui.servicesViewport = viewport.New()
	m.ui.servicesViewport.SetWidth(80)
	m.ui.servicesViewport.SetHeight(3)

	offset := m.calculateScrollOffset()

	assert.Equal(t, 3, offset)
}

func Test_SampleTimelines(t *testing.T) {
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	stopped := newTimeline(terminal.TimelineDefaultSlots)
	stopped.Append(TimelineSlotRunning)

	m := Model{}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api":            {ID: "id-api", Name: "api", Status: model.StatusRunning, Process: model.Process{StartedAt: started}},
		"id-db":             {ID: "id-db", Name: "db", Status: model.StatusStarting, Process: model.Process{StartedAt: started}},
		"id-web":            {ID: "id-web", Name: "web", Status: model.StatusFailed, Process: model.Process{StartedAt: started}},
		"id-queued":         {ID: "id-queued", Name: "queued", Status: model.StatusStarting},
		"id-stopped":        {ID: "id-stopped", Name: "stopped", Status: model.StatusStopped},
		"id-preflight-fail": {ID: "id-preflight-fail", Name: "preflight-fail", Status: model.StatusFailed},
	}}
	m.state.views = map[string]*serviceView{
		"id-api":            {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
		"id-db":             {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
		"id-web":            {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
		"id-queued":         {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
		"id-stopped":        {Timeline: stopped},
		"id-preflight-fail": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
	}

	m.sampleTimelines()

	assert.Equal(t, 1, m.state.views["id-api"].Timeline.count)
	assert.Equal(t, TimelineSlotRunning, m.state.views["id-api"].Timeline.slots()[0])

	assert.Equal(t, 1, m.state.views["id-db"].Timeline.count)
	assert.Equal(t, TimelineSlotStarting, m.state.views["id-db"].Timeline.slots()[0])

	assert.Equal(t, 1, m.state.views["id-web"].Timeline.count)
	assert.Equal(t, TimelineSlotFailed, m.state.views["id-web"].Timeline.slots()[0])

	assert.Equal(t, 0, m.state.views["id-queued"].Timeline.count)

	assert.Equal(t, 2, m.state.views["id-stopped"].Timeline.count)
	assert.Equal(t, TimelineSlotStopped, m.state.views["id-stopped"].Timeline.slots()[1])

	assert.Equal(t, 1, m.state.views["id-preflight-fail"].Timeline.count)
	assert.Equal(t, TimelineSlotFailed, m.state.views["id-preflight-fail"].Timeline.slots()[0])
}

func Test_SampleTimelines_MultipleSamples(t *testing.T) {
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	m := Model{}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api", Status: model.StatusStarting, Process: model.Process{StartedAt: started}},
	}}
	m.state.views = map[string]*serviceView{
		"id-api": {Timeline: newTimeline(terminal.TimelineDefaultSlots)},
	}

	m.sampleTimelines()
	m.snapshot.Services["id-api"].Status = model.StatusRunning
	m.sampleTimelines()
	m.sampleTimelines()

	slots := m.state.views["id-api"].Timeline.slots()
	assert.Equal(t, TimelineSlotStarting, slots[0])
	assert.Equal(t, TimelineSlotRunning, slots[1])
	assert.Equal(t, TimelineSlotRunning, slots[2])
	assert.Equal(t, TimelineSlotEmpty, slots[3])
}

func Test_HandleRestartFailedKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockControl := NewMockControl(ctrl)

	tests := []struct {
		name     string
		before   func() Model
		expected []tea.Msg
	}{
		{
			name: "multiple failed services",
			before: func() Model {
				mockControl.EXPECT().Restart("id-api").Return(services.Admission{}, nil)
				mockControl.EXPECT().Restart("id-db").Return(services.Admission{}, nil)

				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, control: mockControl}
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
					"id-api": {ID: "id-api", Name: "api", Status: model.StatusFailed},
					"id-web": {ID: "id-web", Name: "web", Status: model.StatusRunning},
					"id-db":  {ID: "id-db", Name: "db", Status: model.StatusFailed},
				}}

				return m
			},
			expected: []tea.Msg{
				admissionMsg{name: "api"},
				admissionMsg{name: "db"},
			},
		},
		{
			name: "mixed states - only failed restarted",
			before: func() Model {
				mockControl.EXPECT().Restart("id-db").Return(services.Admission{}, nil)
				mockControl.EXPECT().Restart("id-queue").Return(services.Admission{}, nil)

				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, control: mockControl}
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db", "id-cache", "id-queue"}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
					"id-api":   {ID: "id-api", Name: "api", Status: model.StatusRunning},
					"id-web":   {ID: "id-web", Name: "web", Status: model.StatusStopped},
					"id-db":    {ID: "id-db", Name: "db", Status: model.StatusFailed},
					"id-cache": {ID: "id-cache", Name: "cache", Status: model.StatusStarting},
					"id-queue": {ID: "id-queue", Name: "queue", Status: model.StatusFailed},
				}}

				return m
			},
			expected: []tea.Msg{
				admissionMsg{name: "db"},
				admissionMsg{name: "queue"},
			},
		},
		{
			name: "filter applied - restarts all failed services regardless of filter",
			before: func() Model {
				mockControl.EXPECT().Restart("id-api").Return(services.Admission{}, nil)
				mockControl.EXPECT().Restart("id-web").Return(services.Admission{}, nil)
				mockControl.EXPECT().Restart("id-db").Return(services.Admission{}, nil)

				m := Model{loader: &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}, control: mockControl}
				m.state.serviceIDs = []string{"id-api", "id-web", "id-db"}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
					"id-api": {ID: "id-api", Name: "api", Status: model.StatusFailed},
					"id-web": {ID: "id-web", Name: "web", Status: model.StatusFailed},
					"id-db":  {ID: "id-db", Name: "db", Status: model.StatusFailed},
				}}
				m.state.filterQuery = "api"
				m.state.filteredIDs = []string{"id-api"}

				return m
			},
			expected: []tea.Msg{
				admissionMsg{name: "api"},
				admissionMsg{name: "web"},
				admissionMsg{name: "db"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, cmd := m.handleRestartFailedKey()

			batch := cmd().(tea.BatchMsg)

			require.Len(t, batch, len(tt.expected))

			for i, expected := range tt.expected {
				assert.Equal(t, expected, batch[i]())
			}

			assert.False(t, result.loader.Active)
		})
	}
}

func Test_HandleRestartFailedKey_NothingFailed(t *testing.T) {
	tests := []struct {
		name   string
		before func() Model
	}{
		{
			name: "no failed services",
			before: func() Model {
				m := Model{}
				m.state.serviceIDs = []string{"id-api", "id-web"}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
					"id-api": {ID: "id-api", Name: "api", Status: model.StatusRunning},
					"id-web": {ID: "id-web", Name: "web", Status: model.StatusStopped},
				}}

				return m
			},
		},
		{
			name: "empty services",
			before: func() Model {
				m := Model{}
				m.snapshot = &model.Snapshot{Services: map[string]*model.Service{}}

				return m
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			_, cmd := m.handleRestartFailedKey()

			assert.Nil(t, cmd)
		})
	}
}

func Test_HandleKeyPress_CtrlRRoutesToRestartFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockControl := NewMockControl(ctrl)
	mockControl.EXPECT().Restart("id-api").Return(services.Admission{}, nil)

	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	m := Model{loader: loader, control: mockControl}
	m.state.shuttingDown = false
	m.ui.servicesKeys = defaultKeyMap()
	m.state.serviceIDs = []string{"id-api"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api", Status: model.StatusFailed},
	}}

	msg := tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}

	_, cmd := m.handleKeyPress(msg)

	assert.Equal(t, admissionMsg{name: "api"}, cmd())
}

func Test_HandleKeyPress_ServiceActionKeys(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockControl := NewMockControl(ctrl)

	api := &model.Service{ID: "id-api", Name: "api", Status: model.StatusRunning}
	stopping := services.Admission{Service: model.Service{ID: "id-api", Name: "api"}, Action: contracts.ActionStop, Status: model.StatusStopping}
	restarting := services.Admission{Service: model.Service{ID: "id-api", Name: "api"}, Action: contracts.ActionRestart, Status: model.StatusRestarting}

	m := Model{control: mockControl}
	m.snapshot = &model.Snapshot{
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api}}},
		Services: map[string]*model.Service{"id-api": api},
	}
	m.state.serviceIDs = []string{"id-api"}
	m.ui.servicesKeys = defaultKeyMap()

	tests := []struct {
		name     string
		before   func()
		msg      tea.KeyPressMsg
		expected tea.Msg
	}{
		{
			name: "s toggles the selected service",
			before: func() {
				mockControl.EXPECT().Toggle("id-api").Return(stopping, nil)
			},
			msg:      tea.KeyPressMsg{Code: 's', Text: "s"},
			expected: admissionMsg{name: "api", admission: stopping},
		},
		{
			name: "r restarts the selected service",
			before: func() {
				mockControl.EXPECT().Restart("id-api").Return(restarting, nil)
			},
			msg:      tea.KeyPressMsg{Code: 'r', Text: "r"},
			expected: admissionMsg{name: "api", admission: restarting},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			_, cmd := m.handleKeyPress(tt.msg)

			assert.Equal(t, tt.expected, cmd())
		})
	}
}

func Test_HandleKeyPress_ViewKeys(t *testing.T) {
	m := Model{}
	m.state.asideOpen = true
	m.state.asideTab = AsideTabConfig
	m.ui.showTips = true
	m.ui.servicesKeys = defaultKeyMap()
	m.ui.asideViewport = viewport.New()

	tests := []struct {
		name         string
		msg          tea.KeyPressMsg
		expectedTips bool
		expectedTab  AsideTab
	}{
		{
			name:         "t hides the tips",
			msg:          tea.KeyPressMsg{Code: 't', Text: "t"},
			expectedTips: false,
			expectedTab:  AsideTabConfig,
		},
		{
			name:         "tab moves to the next aside tab",
			msg:          tea.KeyPressMsg{Code: tea.KeyTab},
			expectedTips: true,
			expectedTab:  AsideTabEnv,
		},
		{
			name:         "shift+tab moves to the previous aside tab",
			msg:          tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift},
			expectedTips: true,
			expectedTab:  AsideTabHealth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, cmd := m.handleKeyPress(tt.msg)

			assert.Nil(t, cmd)
			assert.Equal(t, tt.expectedTips, result.ui.showTips)
			assert.Equal(t, tt.expectedTab, result.state.asideTab)
		})
	}
}

func Test_Update_WindowSizeMsg_AsideClosed_FullViewport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	snapshot := &model.Snapshot{Services: map[string]*model.Service{"api": {Name: "api"}}}

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))

	m := Model{registry: mockRegistry}
	m.ui.help = help.New()
	m.ui.servicesViewport = viewport.New()

	teaModel, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := teaModel.(Model)

	assert.Nil(t, cmd)
	assert.Equal(t, 120, result.ui.width)
	assert.Equal(t, 40, result.ui.height)
	assert.Equal(t, 120-terminal.PanelInnerPadding, result.ui.servicesViewport.Width(), "viewport must use full width when aside closed")
	assert.True(t, result.state.ready)
}

func Test_Update_WindowSizeMsg_AsideOpen_SplitViewport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	snapshot := &model.Snapshot{Services: map[string]*model.Service{"api": {Name: "api"}}}

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))

	m := Model{registry: mockRegistry}
	m.ui.asideCache = &asideContentCache{}
	m.state.asideOpen = true
	m.ui.help = help.New()
	m.ui.servicesViewport = viewport.New()

	teaModel, cmd := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	result := teaModel.(Model)

	assert.Nil(t, cmd)
	assert.Equal(t, 200, result.ui.width)
	assert.Equal(t, terminal.AsideMinMainWidth-terminal.PanelInnerPadding, result.ui.servicesViewport.Width(), "viewport must shrink to the auto-fit main width when aside open")
}

func Test_Update_WindowSizeMsg_AsideOpen_NarrowFallbackFullViewport(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	snapshot := &model.Snapshot{Services: map[string]*model.Service{"api": {Name: "api"}}}

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))

	m := Model{registry: mockRegistry}
	m.state.asideOpen = true
	m.ui.help = help.New()
	m.ui.servicesViewport = viewport.New()

	teaModel, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 40})
	result := teaModel.(Model)

	assert.Equal(t, 50-terminal.PanelInnerPadding, result.ui.servicesViewport.Width(), "narrow terminal falls back to full viewport width even with aside open")
	assert.False(t, result.state.asideOpen, "narrow resize auto-closes aside so Esc can not orphan an invisible state")
}

func Test_Update_WindowSizeMsg_AsideOpen_WideKeepsOpen(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	snapshot := &model.Snapshot{Services: map[string]*model.Service{"api": {Name: "api"}}}

	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))

	m := Model{registry: mockRegistry}
	m.ui.asideCache = &asideContentCache{}
	m.state.asideOpen = true
	m.ui.help = help.New()
	m.ui.servicesViewport = viewport.New()

	teaModel, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	result := teaModel.(Model)

	assert.True(t, result.state.asideOpen, "wide-enough terminal preserves asideOpen state")
}

func Test_RecomputeViewport_ReflectsAsideState(t *testing.T) {
	m := Model{snapshot: &model.Snapshot{}}
	m.ui.width = 200
	m.ui.height = 40
	m.ui.servicesViewport = viewport.New()

	m.recomputeViewport()
	assert.Equal(t, 200-terminal.PanelInnerPadding, m.ui.servicesViewport.Width(), "closed aside should use full width")

	m.state.asideOpen = true
	m.recomputeViewport()
	assert.Equal(t, terminal.AsideMinMainWidth-terminal.PanelInnerPadding, m.ui.servicesViewport.Width(), "open aside should shrink viewport to the auto-fit main width")
}

func Test_RecomputeLayout_RespectsAsideState(t *testing.T) {
	m := Model{snapshot: &model.Snapshot{}}
	m.ui.width = 200

	closedLayout := m.recomputeLayout().ui.layout
	m.state.asideOpen = true
	openLayout := m.recomputeLayout().ui.layout

	assert.Greater(t, closedLayout.ContentWidth, openLayout.ContentWidth, "opening aside must shrink the table content width")
}

func Test_HandleKeyPress_EnterOpensAside(t *testing.T) {
	snapshot := &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api"},
		"id-web": {ID: "id-web", Name: "web"},
	}}
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}

	tests := []struct {
		name          string
		before        func() Model
		wantAsideOpen bool
	}{
		{
			name: "Enter on selected service opens aside",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.asideCache = &asideContentCache{}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.width = 200
				m.state.serviceIDs = []string{"id-api", "id-web"}

				return m
			},
			wantAsideOpen: true,
		},
		{
			name: "Enter with no services does nothing",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.width = 200

				return m
			},
			wantAsideOpen: false,
		},
		{
			name: "Enter with selection out of bounds does nothing",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.width = 200
				m.state.serviceIDs = []string{"id-api"}
				m.state.selected = 5

				return m
			},
			wantAsideOpen: false,
		},
		{
			name: "Enter on narrow terminal does not open aside",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.width = 50
				m.state.serviceIDs = []string{"id-api", "id-web"}

				return m
			},
			wantAsideOpen: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, cmd := m.handleKeyPress(enter)

			assert.Nil(t, cmd)
			assert.Equal(t, tt.wantAsideOpen, result.state.asideOpen)
		})
	}
}

func Test_HandleKeyPress_EscClosesAsideBeforeClearingFilter(t *testing.T) {
	api := &model.Service{ID: "id-api", Name: "api"}
	web := &model.Service{ID: "id-web", Name: "web"}
	snapshot := &model.Snapshot{
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api, web}}},
		Services: map[string]*model.Service{"id-api": api, "id-web": web},
	}
	escape := tea.KeyPressMsg{Code: tea.KeyEscape}

	tests := []struct {
		name            string
		before          func() Model
		wantAsideOpen   bool
		wantFilterQuery string
	}{
		{
			name: "Esc closes aside without clearing filter",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.width = 200
				m.state.asideOpen = true
				m.state.filterQuery = "web"
				m.state.filteredIDs = []string{"id-api"}
				m.state.serviceIDs = []string{"id-api", "id-web"}

				return m
			},
			wantAsideOpen:   false,
			wantFilterQuery: "web",
		},
		{
			name: "Esc with aside closed clears filter",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.width = 200
				m.state.filterQuery = "db"
				m.state.filteredIDs = []string{"id-api"}
				m.state.serviceIDs = []string{"id-api", "id-web"}

				return m
			},
			wantAsideOpen:   false,
			wantFilterQuery: "",
		},
		{
			name: "Esc closes aside when no filter active",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.width = 200
				m.state.asideOpen = true
				m.state.serviceIDs = []string{"id-api", "id-web"}

				return m
			},
			wantAsideOpen:   false,
			wantFilterQuery: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, _ := m.handleKeyPress(escape)

			assert.Equal(t, tt.wantAsideOpen, result.state.asideOpen)
			assert.Equal(t, tt.wantFilterQuery, result.state.filterQuery)
		})
	}
}

func Test_HandleKeyPress_UpDownWithAsideOpenAndServicesFocusedMovesSelection(t *testing.T) {
	api := &model.Service{ID: "id-api", Name: "api"}
	web := &model.Service{ID: "id-web", Name: "web"}
	snapshot := &model.Snapshot{
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api, web}}},
		Services: map[string]*model.Service{"id-api": api, "id-web": web},
	}

	tests := []struct {
		name         string
		before       func() Model
		msg          tea.KeyPressMsg
		wantSelected int
	}{
		{
			name: "Down moves selection while aside open and services focused",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.state.asideOpen = true
				m.state.serviceIDs = []string{"id-api", "id-web"}

				return m
			},
			msg:          tea.KeyPressMsg{Code: tea.KeyDown},
			wantSelected: 1,
		},
		{
			name: "Up moves selection while aside open and services focused",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.state.asideOpen = true
				m.state.serviceIDs = []string{"id-api", "id-web"}
				m.state.selected = 1

				return m
			},
			msg:          tea.KeyPressMsg{Code: tea.KeyUp},
			wantSelected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, _ := m.handleKeyPress(tt.msg)

			assert.True(t, result.state.asideOpen, "aside should remain open after navigation")
			assert.Equal(t, tt.wantSelected, result.state.selected)
		})
	}
}

func Test_HandleAsideTab_ResetsAsideScroll(t *testing.T) {
	tests := []struct {
		name    string
		before  func() Model
		handler func(Model) (Model, tea.Cmd)
		wantTab AsideTab
	}{
		{
			name: "next tab resets scroll",
			before: func() Model {
				m := Model{}
				m.ui.asideViewport = viewport.New()
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
				m.ui.asideViewport.GotoBottom()
				m.state.asideTab = AsideTabConfig

				require.Positive(t, m.ui.asideViewport.YOffset(), "precondition: viewport must be scrolled down")

				return m
			},
			handler: Model.handleAsideTabNext,
			wantTab: AsideTabEnv,
		},
		{
			name: "prev tab resets scroll",
			before: func() Model {
				m := Model{}
				m.ui.asideViewport = viewport.New()
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
				m.ui.asideViewport.GotoBottom()
				m.state.asideTab = AsideTabConfig

				require.Positive(t, m.ui.asideViewport.YOffset(), "precondition: viewport must be scrolled down")

				return m
			},
			handler: Model.handleAsideTabPrev,
			wantTab: AsideTabHealth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, _ := tt.handler(m)

			assert.Equal(t, tt.wantTab, result.state.asideTab)
			assert.Equal(t, 0, result.ui.asideViewport.YOffset(), "aside scroll must reset to top after tab change")
		})
	}
}

func Test_HandleSelectionKeys_ServicesFocusedMovesSelectionAndResetsAsideScroll(t *testing.T) {
	api := &model.Service{ID: "id-api", Name: "api"}
	web := &model.Service{ID: "id-web", Name: "web"}
	snapshot := &model.Snapshot{
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api, web}}},
		Services: map[string]*model.Service{"id-api": api, "id-web": web},
	}

	tests := []struct {
		name    string
		before  func() Model
		msg     tea.KeyPressMsg
		wantSel int
	}{
		{
			name: "down moves selection and resets aside scroll",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.asideCache = &asideContentCache{}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
				m.ui.asideViewport.GotoBottom()
				m.state.asideOpen = true
				m.state.serviceIDs = []string{"id-api", "id-web"}

				require.Positive(t, m.ui.asideViewport.YOffset(), "precondition: viewport must be scrolled down")

				return m
			},
			msg:     tea.KeyPressMsg{Code: tea.KeyDown},
			wantSel: 1,
		},
		{
			name: "up moves selection and resets aside scroll",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.asideCache = &asideContentCache{}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
				m.ui.asideViewport.GotoBottom()
				m.state.asideOpen = true
				m.state.serviceIDs = []string{"id-api", "id-web"}
				m.state.selected = 1

				require.Positive(t, m.ui.asideViewport.YOffset(), "precondition: viewport must be scrolled down")

				return m
			},
			msg:     tea.KeyPressMsg{Code: tea.KeyUp},
			wantSel: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, _ := m.handleKeyPress(tt.msg)

			assert.Equal(t, tt.wantSel, result.state.selected)
			assert.Equal(t, 0, result.ui.asideViewport.YOffset(), "aside scroll must reset when service selection changes")
		})
	}
}

func Test_HandleKeyPress_AsideFocusedScrollsAsideViewport(t *testing.T) {
	api := &model.Service{ID: "id-api", Name: "api"}
	snapshot := &model.Snapshot{
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api}}},
		Services: map[string]*model.Service{"id-api": api},
	}

	tests := []struct {
		name       string
		before     func() Model
		msg        tea.KeyPressMsg
		wantOffset int
	}{
		{
			name: "down scrolls aside viewport down",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
				m.state.asideOpen = true
				m.state.asideFocused = true
				m.state.serviceIDs = []string{"id-api"}

				return m
			},
			msg:        tea.KeyPressMsg{Code: tea.KeyDown},
			wantOffset: 1,
		},
		{
			name: "up scrolls aside viewport up",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
				m.ui.asideViewport.SetYOffset(46)
				m.state.asideOpen = true
				m.state.asideFocused = true
				m.state.serviceIDs = []string{"id-api"}

				return m
			},
			msg:        tea.KeyPressMsg{Code: tea.KeyUp},
			wantOffset: 45,
		},
		{
			name: "end jumps to bottom of aside viewport",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
				m.state.asideOpen = true
				m.state.asideFocused = true
				m.state.serviceIDs = []string{"id-api"}

				return m
			},
			msg:        tea.KeyPressMsg{Code: tea.KeyEnd},
			wantOffset: 46,
		},
		{
			name: "home jumps to top of aside viewport",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
				m.ui.asideViewport.SetYOffset(46)
				m.state.asideOpen = true
				m.state.asideFocused = true
				m.state.serviceIDs = []string{"id-api"}

				return m
			},
			msg:        tea.KeyPressMsg{Code: tea.KeyHome},
			wantOffset: 0,
		},
		{
			name: "pgdown scrolls aside viewport down",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
				m.state.asideOpen = true
				m.state.asideFocused = true
				m.state.serviceIDs = []string{"id-api"}

				return m
			},
			msg:        tea.KeyPressMsg{Code: tea.KeyPgDown},
			wantOffset: 5,
		},
		{
			name: "pgup scrolls aside viewport up",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
				m.ui.asideViewport.SetYOffset(46)
				m.state.asideOpen = true
				m.state.asideFocused = true
				m.state.serviceIDs = []string{"id-api"}

				return m
			},
			msg:        tea.KeyPressMsg{Code: tea.KeyPgUp},
			wantOffset: 41,
		},
		{
			name: "another key leaves the aside viewport alone",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.ui.asideViewport.SetWidth(40)
				m.ui.asideViewport.SetHeight(5)
				m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
				m.ui.asideViewport.SetYOffset(10)
				m.state.asideOpen = true
				m.state.asideFocused = true
				m.state.serviceIDs = []string{"id-api"}

				return m
			},
			msg:        tea.KeyPressMsg{Code: 'x', Text: "x"},
			wantOffset: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, _ := m.handleKeyPress(tt.msg)

			assert.Equal(t, tt.wantOffset, result.ui.asideViewport.YOffset())
			assert.Equal(t, 0, result.state.selected, "service selection must not move when aside is focused")
			assert.Equal(t, 0, result.ui.servicesViewport.YOffset(), "services viewport must not move when aside is focused")
		})
	}
}

func Test_HandleKeyPress_ScrollKeysWhenAsideClosedRouteToServicesViewport(t *testing.T) {
	m := Model{}
	m.ui.servicesKeys = defaultKeyMap()
	m.ui.servicesViewport = viewport.New()
	m.ui.servicesViewport.SetWidth(40)
	m.ui.servicesViewport.SetHeight(5)
	m.ui.servicesViewport.SetContent(strings.Repeat("line\n", 50))
	m.ui.asideViewport = viewport.New()

	m.state.asideOpen = false
	m.state.serviceIDs = []string{"id-api"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"id-api": {ID: "id-api", Name: "api"}}}
	m.snapshot.Tiers = []*model.Tier{{Name: "tier1", Services: []*model.Service{m.snapshot.Services["id-api"]}}}

	msg := tea.KeyPressMsg{Code: tea.KeyPgDown}

	result, _ := m.handleKeyPress(msg)

	assert.Positive(t, result.ui.servicesViewport.YOffset(), "services viewport must scroll when aside is closed")
	assert.Equal(t, 0, result.ui.asideViewport.YOffset(), "aside viewport must stay put when aside is closed")
}

func Test_SetAsideOpen_SetsFocusAndResetsAsideScroll(t *testing.T) {
	m := Model{}
	m.ui.asideCache = &asideContentCache{}
	m.ui.width = 200
	m.ui.height = 40
	m.ui.servicesViewport = viewport.New()
	m.ui.asideViewport = viewport.New()
	m.ui.asideViewport.SetWidth(40)
	m.ui.asideViewport.SetHeight(5)
	m.ui.asideViewport.SetContent(strings.Repeat("line\n", 50))
	m.ui.asideViewport.GotoBottom()
	require.Positive(t, m.ui.asideViewport.YOffset(), "precondition: viewport must be scrolled down")

	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"api": {Name: "api"}}}

	opened := m.setAsideOpen(true)

	assert.True(t, opened.state.asideOpen)
	assert.True(t, opened.state.asideFocused, "opening aside transfers focus to aside")
	assert.Equal(t, 0, opened.ui.asideViewport.YOffset(), "opening aside resets viewport scroll to top")
}

func Test_SetAsideOpen_ClosingReturnsFocusToServices(t *testing.T) {
	m := Model{}
	m.state.asideOpen = true
	m.state.asideFocused = true
	m.ui.width = 200
	m.ui.height = 40
	m.ui.servicesViewport = viewport.New()
	m.ui.asideViewport = viewport.New()
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{"api": {Name: "api"}}}

	closed := m.setAsideOpen(false)

	assert.False(t, closed.state.asideOpen)
	assert.False(t, closed.state.asideFocused, "closing aside transfers focus back to services")
}

func Test_HandleFocusToggle(t *testing.T) {
	api := &model.Service{ID: "id-api", Name: "api"}
	snapshot := &model.Snapshot{
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api}}},
		Services: map[string]*model.Service{"id-api": api},
	}
	toggle := tea.KeyPressMsg{Code: '\\', Text: "\\"}

	tests := []struct {
		name        string
		before      func() Model
		wantFocused bool
	}{
		{
			name: "toggle while aside-focused moves focus to services",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.state.asideOpen = true
				m.state.asideFocused = true
				m.state.serviceIDs = []string{"id-api"}

				return m
			},
			wantFocused: false,
		},
		{
			name: "toggle while services-focused moves focus to aside",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.state.asideOpen = true
				m.state.serviceIDs = []string{"id-api"}

				return m
			},
			wantFocused: true,
		},
		{
			name: "toggle while aside is closed is a no-op",
			before: func() Model {
				m := Model{snapshot: snapshot}
				m.ui.servicesKeys = defaultKeyMap()
				m.ui.servicesViewport = viewport.New()
				m.ui.asideViewport = viewport.New()
				m.state.serviceIDs = []string{"id-api"}

				return m
			},
			wantFocused: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result, _ := m.handleKeyPress(toggle)

			assert.Equal(t, tt.wantFocused, result.state.asideFocused)
		})
	}
}

func Test_SetAsideOpen_PopulatesAsideViewportContent(t *testing.T) {
	m := Model{theme: terminal.NewTheme(terminal.AppearanceDark)}
	m.ui.asideCache = &asideContentCache{}
	m.ui.width = 200
	m.ui.height = 30
	m.ui.servicesViewport = viewport.New()
	m.ui.asideViewport = viewport.New()

	m.state.serviceIDs = []string{"id-api"}
	m.snapshot = &model.Snapshot{Services: map[string]*model.Service{
		"id-api": {ID: "id-api", Name: "api", Tier: "foundation", Directory: "services/api", Command: "go run main.go", Status: model.StatusRunning},
	}}
	m.snapshot.Tiers = []*model.Tier{{Name: "foundation", Services: []*model.Service{m.snapshot.Services["id-api"]}}}

	opened := m.setAsideOpen(true)

	got := opened.ui.asideViewport.GetContent()
	assert.Contains(t, got, "services/api", "setAsideOpen must populate aside viewport content via updateAsideContent")
}

func Test_PanelBorderStyle_FollowsFocus(t *testing.T) {
	tests := []struct {
		name         string
		before       func() Model
		wantServices bool
		wantAside    bool
	}{
		{
			name: "aside closed focuses services",
			before: func() Model {
				return Model{}
			},
			wantServices: true,
			wantAside:    false,
		},
		{
			name: "aside open and aside focused",
			before: func() Model {
				m := Model{}
				m.state.asideOpen = true
				m.state.asideFocused = true

				return m
			},
			wantServices: false,
			wantAside:    true,
		},
		{
			name: "aside open and services focused",
			before: func() Model {
				m := Model{}
				m.state.asideOpen = true

				return m
			},
			wantServices: true,
			wantAside:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			services := m.servicesPanelBorderStyle()
			aside := m.asidePanelBorderStyle()

			assert.Equal(t, tt.wantServices, services.GetForeground() == terminal.PanelBorderStyle.GetForeground(), "services panel border")
			assert.Equal(t, tt.wantAside, aside.GetForeground() == terminal.PanelBorderStyle.GetForeground(), "aside panel border")
		})
	}
}

func Test_RecomputeViewport_SizesAsideViewport(t *testing.T) {
	m := Model{snapshot: &model.Snapshot{}}
	m.ui.width = 200
	m.ui.height = 40
	m.ui.servicesViewport = viewport.New()
	m.ui.asideViewport = viewport.New()

	m.state.asideOpen = true
	m.recomputeViewport()

	_, asideWidth := m.panelWidths()
	expectedWidth := max(asideWidth-terminal.PanelInnerPadding, 0)
	assert.Equal(t, expectedWidth, m.ui.asideViewport.Width(), "open aside should size viewport to the aside inner width")
	assert.Positive(t, m.ui.asideViewport.Height())
}

func Test_Update_Tick(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)
	mockMonitor := NewMockMonitor(ctrl)

	log := slog.New(slog.DiscardHandler)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	api := &model.Service{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusPending}
	db := &model.Service{ID: "id-db", Name: "db", Tier: "tier1", Status: model.StatusPending}
	web := &model.Service{ID: "id-web", Name: "web", Tier: "tier1", Status: model.StatusPending}
	pending := &model.Snapshot{
		Phase:    model.PhaseStartup,
		Profile:  "dev",
		Resolved: true,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api, db, web}}},
		Services: map[string]*model.Service{"id-api": api, "id-db": db, "id-web": web},
	}
	runningAPI := &model.Service{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusRunning, Process: model.Process{PID: 42, StartedAt: t0}, AttemptedAt: t0, LifecycleAt: t0}
	running := &model.Snapshot{
		Phase:    model.PhaseRunning,
		Profile:  "dev",
		Resolved: true,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{runningAPI, db, web}}},
		Services: map[string]*model.Service{"id-api": runningAPI, "id-db": db, "id-web": web},
	}
	stoppedAPI := &model.Service{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusStopped}
	stopped := &model.Snapshot{
		Phase:    model.PhaseStopped,
		Profile:  "dev",
		Resolved: true,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{stoppedAPI, db, web}}},
		Services: map[string]*model.Service{"id-api": stoppedAPI, "id-db": db, "id-web": web},
	}

	tests := []struct {
		name        string
		before      func() Model
		wantStatus  model.Status
		wantSamples int
		wantFirst   tea.Msg
		wantCounter int
	}{
		{
			name: "a dropped notification is recovered by the periodic read",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(running))
				mockMonitor.EXPECT().GetStats(gomock.Any(), os.Getpid()).Return(resources.Stats{}, nil)

				m := Model{ctx: t.Context(), registry: mockRegistry, monitor: mockMonitor, loader: NewLoader(), snapshot: pending, log: log}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)
				m.applySnapshot()

				return m
			},
			wantStatus:  model.StatusRunning,
			wantSamples: 1,
			wantFirst:   appStatsMsg{},
			wantCounter: 1,
		},
		{
			name: "an unchanged state is read again without effect",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(pending))
				mockMonitor.EXPECT().GetStats(gomock.Any(), os.Getpid()).Return(resources.Stats{}, nil)

				m := Model{ctx: t.Context(), registry: mockRegistry, monitor: mockMonitor, loader: NewLoader(), snapshot: pending, log: log}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)
				m.applySnapshot()

				return m
			},
			wantStatus:  model.StatusPending,
			wantFirst:   appStatsMsg{},
			wantCounter: 1,
		},
		{
			name: "the stopped phase quits the program",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(stopped))

				m := Model{ctx: t.Context(), registry: mockRegistry, monitor: mockMonitor, loader: NewLoader(), snapshot: pending, log: log}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)
				m.applySnapshot()

				return m
			},
			wantStatus:  model.StatusStopped,
			wantSamples: 1,
			wantFirst:   tea.QuitMsg{},
			wantCounter: 1,
		},
		{
			name: "the tick counter wraps at its maximum and refreshes",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(running))
				mockMonitor.EXPECT().GetStats(gomock.Any(), os.Getpid()).Return(resources.Stats{}, nil)

				m := Model{ctx: t.Context(), registry: mockRegistry, monitor: mockMonitor, loader: NewLoader(), snapshot: pending, log: log}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)
				m.applySnapshot()
				m.state.now = t0
				m.ui.tickCounter = tickCounterMaximum - 1

				return m
			},
			wantStatus:  model.StatusRunning,
			wantSamples: 1,
			wantFirst:   appStatsMsg{},
			wantCounter: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			teaModel, cmd := m.Update(tickMsg(t0))
			result := teaModel.(Model)

			assert.Equal(t, tt.wantStatus, result.state.views["id-api"].Status)
			assert.Equal(t, tt.wantSamples, result.state.views["id-api"].Timeline.Count())
			assert.False(t, result.state.now.IsZero())
			assert.Equal(t, tt.wantFirst, cmd().(tea.BatchMsg)[0]())
			assert.Equal(t, tt.wantCounter, result.ui.tickCounter)
		})
	}
}

func Test_tickCmd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		msg := tickCmd()()

		assert.IsType(t, tickMsg{}, msg)
	})
}

func Test_Update_PreflightBeforeTheResolvedSnapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	log := slog.New(slog.DiscardHandler)

	api := &model.Service{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusPending}
	db := &model.Service{ID: "id-db", Name: "db", Tier: "tier1", Status: model.StatusPending}
	web := &model.Service{ID: "id-web", Name: "web", Tier: "tier1", Status: model.StatusPending}
	empty := &model.Snapshot{}
	resolved := &model.Snapshot{
		Phase:    model.PhaseStartup,
		Profile:  "dev",
		Resolved: true,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api, db, web}}},
		Services: map[string]*model.Service{"id-api": api, "id-db": db, "id-web": web},
	}

	started := contracts.Message{Type: contracts.EventPreflightStarted, Data: contracts.PreflightStarted{}}
	changed := contracts.Message{Type: contracts.EventSnapshotChanged, Data: contracts.SnapshotChanged{}}
	complete := contracts.Message{Type: contracts.EventPreflightComplete, Data: contracts.PreflightComplete{}}

	gomock.InOrder(
		mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(empty)),
		mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(resolved)),
		mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(resolved)),
	)

	m := Model{log: log, loader: NewLoader(), registry: mockRegistry}
	m.state.views = make(map[string]*serviceView)
	m.state.restarting = make(map[string]bool)

	teaModel, _ := m.Update(EventMsg(started))
	teaModel, _ = teaModel.Update(EventMsg(changed))
	afterResolved := teaModel.(Model)
	scanning := afterResolved.loader.Message()

	teaModel, _ = teaModel.Update(EventMsg(complete))
	afterComplete := teaModel.(Model)

	assert.Equal(t, "preflight: scanning processes…", scanning)
	assert.True(t, afterResolved.state.resolved)
	assert.False(t, afterComplete.loader.Active)
}
