package tui

import (
	"log/slog"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_HandlePreflightStarted(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}

	result := m.handlePreflightStarted()

	assert.True(t, result.loader.Active)
	assert.True(t, result.loader.Has(loaderKeyPreflight))
	assert.Equal(t, "preflight: scanning processes\u2026", result.loader.Message())
}

func Test_HandlePreflightKill(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start(loaderKeyPreflight, "preflight: scanning processes…")

	m := Model{loader: loader}

	event := contracts.Message{Type: contracts.EventPreflightKilled, Data: contracts.PreflightKilled{Service: "api", PID: 1234, Name: "node"}}

	result := m.handlePreflightKill(event)

	assert.True(t, result.loader.Active)
	assert.Equal(t, "preflight: stopping api\u2026", result.loader.Message())
}

func Test_HandlePreflightKill_InvalidData(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start(loaderKeyPreflight, "preflight: scanning processes…")

	m := Model{loader: loader}

	event := contracts.Message{Type: contracts.EventPreflightKilled, Data: "invalid"}

	result := m.handlePreflightKill(event)

	assert.True(t, result.loader.Active)
	assert.Equal(t, "preflight: scanning processes\u2026", result.loader.Message())
}

func Test_HandlePreflightComplete(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start(loaderKeyPreflight, "preflight: stopping api...")

	m := Model{loader: loader}

	result := m.handlePreflightComplete()

	assert.False(t, result.loader.Active)
	assert.False(t, result.loader.Has(loaderKeyPreflight))
}

func Test_HandleSignal(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	m := Model{loader: loader}
	m.state.shuttingDown = false

	result := m.handleSignal()

	assert.True(t, result.state.shuttingDown)
	assert.True(t, result.loader.Active)
	assert.Equal(t, "shutting down all services\u2026", result.loader.Message())
}

func Test_HandleUpdateAvailable(t *testing.T) {
	tests := []struct {
		name        string
		before      func() Model
		msg         contracts.Message
		wantVersion string
	}{
		{
			name: "stores version from valid payload",
			before: func() Model {
				return Model{}
			},
			msg:         contracts.Message{Type: contracts.EventUpdateAvailable, Data: contracts.UpdateAvailable{Version: "v0.20.0"}},
			wantVersion: "v0.20.0",
		},
		{
			name: "ignores wrong payload type",
			before: func() Model {
				return Model{}
			},
			msg:         contracts.Message{Type: contracts.EventUpdateAvailable, Data: "invalid"},
			wantVersion: "",
		},
		{
			name: "ignores wrong payload type but keeps prior version",
			before: func() Model {
				m := Model{}
				m.state.availableVersion = "v0.19.5"

				return m
			},
			msg:         contracts.Message{Type: contracts.EventUpdateAvailable, Data: 42},
			wantVersion: "v0.19.5",
		},
		{
			name: "overwrites prior version with newer payload",
			before: func() Model {
				m := Model{}
				m.state.availableVersion = "v0.20.0"

				return m
			},
			msg:         contracts.Message{Type: contracts.EventUpdateAvailable, Data: contracts.UpdateAvailable{Version: "v1.0.0"}},
			wantVersion: "v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.handleUpdateAvailable(tt.msg)

			assert.Equal(t, tt.wantVersion, result.state.availableVersion)
		})
	}
}

func Test_ApplySnapshot(t *testing.T) {
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
	reversed := &model.Snapshot{
		Phase:    model.PhaseStartup,
		Profile:  "dev",
		Resolved: true,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{web, db, api}}},
		Services: map[string]*model.Service{"id-api": api, "id-db": db, "id-web": web},
	}
	runningAPI := &model.Service{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusRunning, Process: model.Process{PID: 42, StartedAt: t0}, AttemptedAt: t0, LifecycleAt: t0.Add(time.Second)}
	failedDB := &model.Service{ID: "id-db", Name: "db", Tier: "tier1", Status: model.StatusFailed, Error: "boom"}
	changed := &model.Snapshot{
		Phase:    model.PhaseRunning,
		Profile:  "dev",
		Resolved: true,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{runningAPI, failedDB, web}}},
		Services: map[string]*model.Service{"id-api": runningAPI, "id-db": failedDB, "id-web": web},
	}
	stoppedAPI := &model.Service{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusStopped}
	stoppedDB := &model.Service{ID: "id-db", Name: "db", Tier: "tier1", Status: model.StatusStopped}
	stopped := &model.Snapshot{
		Phase:    model.PhaseStopped,
		Profile:  "dev",
		Resolved: true,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{stoppedAPI, stoppedDB, web}}},
		Services: map[string]*model.Service{"id-api": stoppedAPI, "id-db": stoppedDB, "id-web": web},
	}

	tests := []struct {
		name       string
		before     func() Model
		wantQuit   bool
		wantIDs    []string
		wantStatus map[string]model.Status
		wantLoader bool
	}{
		{
			name: "the first resolved snapshot builds the view state in tier order",
			before: func() Model {
				m := Model{log: log, loader: NewLoader(), snapshot: reversed}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)

				return m
			},
			wantIDs:    []string{"id-web", "id-db", "id-api"},
			wantStatus: map[string]model.Status{"id-web": model.StatusPending, "id-db": model.StatusPending, "id-api": model.StatusPending},
		},
		{
			name: "a change moves the statuses the view saw",
			before: func() Model {
				m := Model{log: log, loader: NewLoader(), snapshot: pending}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)
				m.applySnapshot()
				m.snapshot = changed

				return m
			},
			wantIDs:    []string{"id-api", "id-db", "id-web"},
			wantStatus: map[string]model.Status{"id-api": model.StatusRunning, "id-db": model.StatusFailed, "id-web": model.StatusPending},
		},
		{
			name: "an unchanged snapshot runs no effect",
			before: func() Model {
				m := Model{log: log, loader: NewLoader(), snapshot: pending}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)
				m.applySnapshot()
				m.loader.Start("id-api", "starting api…")

				return m
			},
			wantIDs:    []string{"id-api", "id-db", "id-web"},
			wantStatus: map[string]model.Status{"id-api": model.StatusPending, "id-db": model.StatusPending, "id-web": model.StatusPending},
			wantLoader: true,
		},
		{
			name: "the stopped phase clears the loader and quits",
			before: func() Model {
				m := Model{log: log, loader: NewLoader(), snapshot: pending}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)
				m.applySnapshot()
				m.loader.Start(loaderKeyShutdown, "shutting down all services…")
				m.snapshot = stopped

				return m
			},
			wantQuit:   true,
			wantIDs:    []string{"id-api", "id-db", "id-web"},
			wantStatus: map[string]model.Status{"id-api": model.StatusStopped, "id-db": model.StatusStopped, "id-web": model.StatusPending},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			cmd := m.applySnapshot()

			assert.Equal(t, tt.wantQuit, cmd != nil)
			assert.Equal(t, tt.wantIDs, m.state.serviceIDs)
			assert.Equal(t, tt.wantLoader, m.loader.Active)

			for id, status := range tt.wantStatus {
				assert.Equal(t, status, m.state.views[id].Status, id)
				assert.NotNil(t, m.state.views[id].Timeline, id)
				assert.NotNil(t, m.state.views[id].Blink, id)
			}
		})
	}
}

func Test_ApplyService(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		before         func() Model
		view           *serviceView
		service        *model.Service
		wantStatus     model.Status
		wantLoader     string
		wantRestarting bool
		wantAmber      int
	}{
		{
			name: "starting opens the attempt and starts the loader",
			before: func() Model {
				m := Model{loader: NewLoader()}
				m.state.restarting = map[string]bool{}

				return m
			},
			view:       &serviceView{Status: model.StatusPending, Timeline: newTimeline()},
			service:    &model.Service{ID: "id-api", Name: "api", Status: model.StatusStarting, Process: model.Process{PID: 42, StartedAt: t0}, AttemptedAt: t0},
			wantStatus: model.StatusStarting,
			wantLoader: "starting api…",
		},
		{
			name: "starting keeps the optimistic loader message",
			before: func() Model {
				m := Model{loader: &Loader{Model: spinner.New(), Active: true, queue: []LoaderItem{{Service: "id-api", Message: "restarting api…"}}}}
				m.state.restarting = map[string]bool{}

				return m
			},
			view:       &serviceView{Status: model.StatusStopped, Timeline: newTimeline()},
			service:    &model.Service{ID: "id-api", Name: "api", Status: model.StatusStarting, AttemptedAt: t0},
			wantStatus: model.StatusStarting,
			wantLoader: "restarting api…",
		},
		{
			name: "running stops the loader and backfills the startup history of the attempt",
			before: func() Model {
				m := Model{loader: &Loader{Model: spinner.New(), Active: true, queue: []LoaderItem{{Service: "id-api", Message: "starting api…"}}}}
				m.state.restarting = map[string]bool{}

				return m
			},
			view:       &serviceView{Status: model.StatusStarting, StartupActive: true, AttemptedAt: t0, Timeline: newTimeline()},
			service:    &model.Service{ID: "id-api", Name: "api", Status: model.StatusRunning, Process: model.Process{PID: 42, StartedAt: t0}, AttemptedAt: t0, LifecycleAt: t0.Add(3 * time.Second)},
			wantStatus: model.StatusRunning,
			wantAmber:  3,
		},
		{
			name: "running after a start the view never saw backfills from the snapshot's attempt",
			before: func() Model {
				m := Model{loader: NewLoader()}
				m.state.restarting = map[string]bool{}

				return m
			},
			view:       &serviceView{Status: model.StatusPending, Timeline: newTimeline()},
			service:    &model.Service{ID: "id-api", Name: "api", Status: model.StatusRunning, Process: model.Process{PID: 42, StartedAt: t0}, AttemptedAt: t0, LifecycleAt: t0.Add(2 * time.Second)},
			wantStatus: model.StatusRunning,
			wantAmber:  2,
		},
		{
			name: "failed stops the loader",
			before: func() Model {
				m := Model{loader: &Loader{Model: spinner.New(), Active: true, queue: []LoaderItem{{Service: "id-api", Message: "starting api…"}}}}
				m.state.restarting = map[string]bool{}

				return m
			},
			view:       &serviceView{Status: model.StatusStarting, StartupActive: true, AttemptedAt: t0, Timeline: newTimeline()},
			service:    &model.Service{ID: "id-api", Name: "api", Status: model.StatusFailed, Error: "readiness timeout", AttemptedAt: t0, LifecycleAt: t0.Add(time.Second)},
			wantStatus: model.StatusFailed,
			wantAmber:  1,
		},
		{
			name: "restarting flags the restart and starts the loader",
			before: func() Model {
				m := Model{loader: NewLoader()}
				m.state.restarting = map[string]bool{}

				return m
			},
			view:           &serviceView{Status: model.StatusRunning, AttemptedAt: t0, Timeline: newTimeline()},
			service:        &model.Service{ID: "id-api", Name: "api", Status: model.StatusRestarting, AttemptedAt: t0},
			wantStatus:     model.StatusRestarting,
			wantLoader:     "restarting api…",
			wantRestarting: true,
		},
		{
			name: "stopped during a restart keeps the loader",
			before: func() Model {
				m := Model{loader: &Loader{Model: spinner.New(), Active: true, queue: []LoaderItem{{Service: "id-api", Message: "restarting api…"}}}}
				m.state.restarting = map[string]bool{"id-api": true}

				return m
			},
			view:           &serviceView{Status: model.StatusRestarting, AttemptedAt: t0, Timeline: newTimeline()},
			service:        &model.Service{ID: "id-api", Name: "api", Status: model.StatusStopped, AttemptedAt: t0},
			wantStatus:     model.StatusStopped,
			wantLoader:     "restarting api…",
			wantRestarting: true,
		},
		{
			name: "stopped outside a restart stops the loader",
			before: func() Model {
				m := Model{loader: &Loader{Model: spinner.New(), Active: true, queue: []LoaderItem{{Service: "id-api", Message: "stopping api…"}}}}
				m.state.restarting = map[string]bool{}

				return m
			},
			view:       &serviceView{Status: model.StatusStopping, AttemptedAt: t0, Timeline: newTimeline()},
			service:    &model.Service{ID: "id-api", Name: "api", Status: model.StatusStopped, AttemptedAt: t0},
			wantStatus: model.StatusStopped,
		},
		{
			name: "an unchanged status keeps the optimistic loader",
			before: func() Model {
				m := Model{loader: &Loader{Model: spinner.New(), Active: true, queue: []LoaderItem{{Service: "id-api", Message: "stopping api…"}}}}
				m.state.restarting = map[string]bool{}

				return m
			},
			view:       &serviceView{Status: model.StatusRunning, AttemptedAt: t0, Timeline: newTimeline()},
			service:    &model.Service{ID: "id-api", Name: "api", Status: model.StatusRunning, Process: model.Process{PID: 42, CPU: 12.5, Memory: 256 * 1024 * 1024}, AttemptedAt: t0},
			wantStatus: model.StatusRunning,
			wantLoader: "stopping api…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			m.applyService(tt.view, tt.service)

			assert.Equal(t, tt.wantStatus, tt.view.Status)
			assert.Equal(t, tt.service.AttemptedAt, tt.view.AttemptedAt)
			assert.Equal(t, tt.wantLoader, m.loader.Message())
			assert.Equal(t, tt.wantRestarting, m.state.restarting[tt.service.ID])
			assert.Equal(t, tt.wantAmber, tt.view.Timeline.Count())
			assert.Equal(t, tt.wantStatus == model.StatusStarting, tt.view.StartupActive)
		})
	}
}

func Test_ApplySnapshot_LateAttachment(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	api := &model.Service{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusRunning, Process: model.Process{PID: 42, StartedAt: t0}, AttemptedAt: t0, LifecycleAt: t0.Add(5 * time.Second)}
	db := &model.Service{ID: "id-db", Name: "db", Tier: "tier1", Status: model.StatusStarting, Process: model.Process{PID: 43, StartedAt: t0}, AttemptedAt: t0, LifecycleAt: t0}
	web := &model.Service{ID: "id-web", Name: "web", Tier: "tier1", Status: model.StatusRestarting, AttemptedAt: t0, LifecycleAt: t0.Add(time.Second)}
	snapshot := &model.Snapshot{
		Phase:    model.PhaseRunning,
		Profile:  "dev",
		Resolved: true,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api, db, web}}},
		Services: map[string]*model.Service{"id-api": api, "id-db": db, "id-web": web},
	}

	m := Model{log: log, loader: NewLoader(), snapshot: snapshot}
	m.state.views = make(map[string]*serviceView)
	m.state.restarting = make(map[string]bool)

	cmd := m.applySnapshot()

	assert.Nil(t, cmd)
	assert.Equal(t, []string{"id-api", "id-db", "id-web"}, m.state.serviceIDs)
	assert.Equal(t, model.StatusRunning, m.state.views["id-api"].Status)
	assert.Equal(t, 5, m.state.views["id-api"].Timeline.Count(), "the startup the view never saw is backfilled from the snapshot")
	assert.False(t, m.loader.Has("id-api"))
	assert.Equal(t, model.StatusStarting, m.state.views["id-db"].Status)
	assert.True(t, m.state.views["id-db"].StartupActive)
	assert.True(t, m.loader.Has("id-db"))
	assert.Equal(t, model.StatusRestarting, m.state.views["id-web"].Status)
	assert.True(t, m.state.restarting["id-web"])
	assert.True(t, m.loader.Has("id-web"))
	assert.Equal(t, "starting db…", m.loader.Message())
}

func Test_ApplySnapshot_Transitions(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(10 * time.Second)

	pendingAPI := &model.Service{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusPending}
	pending := &model.Snapshot{
		Phase:    model.PhaseStartup,
		Profile:  "dev",
		Resolved: true,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{pendingAPI}}},
		Services: map[string]*model.Service{"id-api": pendingAPI},
	}

	tests := []struct {
		name           string
		statuses       []model.Service
		sampledBetween int
		wantStatus     model.Status
		wantLoader     bool
		wantRestarting bool
		wantAmber      int
		wantSampled    int
	}{
		{
			name: "restarting to running with the stopped and starting states never read",
			statuses: []model.Service{
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusRunning, AttemptedAt: t0, LifecycleAt: t0},
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusRestarting, AttemptedAt: t0, LifecycleAt: t1},
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusRunning, AttemptedAt: t1, LifecycleAt: t1.Add(2 * time.Second)},
			},
			wantStatus:  model.StatusRunning,
			wantAmber:   2,
			wantSampled: 2,
		},
		{
			name: "a retry keeps the status starting but opens a new attempt",
			statuses: []model.Service{
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusStarting, AttemptedAt: t0, LifecycleAt: t0},
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusStarting, AttemptedAt: t1, LifecycleAt: t1},
			},
			sampledBetween: 3,
			wantStatus:     model.StatusStarting,
			wantLoader:     true,
			wantSampled:    0,
		},
		{
			name: "restarting to stopped keeps the loader and starting clears the restart flag",
			statuses: []model.Service{
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusRunning, AttemptedAt: t0, LifecycleAt: t0},
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusRestarting, AttemptedAt: t0, LifecycleAt: t1},
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusStopped, AttemptedAt: t0, LifecycleAt: t1},
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusStarting, AttemptedAt: t1, LifecycleAt: t1},
			},
			wantStatus: model.StatusStarting,
			wantLoader: true,
		},
		{
			name: "starting to failed with the same attempt backfills the attempt",
			statuses: []model.Service{
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusStarting, AttemptedAt: t0, LifecycleAt: t0},
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusFailed, AttemptedAt: t0, LifecycleAt: t0.Add(4 * time.Second)},
			},
			sampledBetween: 1,
			wantStatus:     model.StatusFailed,
			wantAmber:      3,
			wantSampled:    4,
		},
		{
			name: "running to stopped without an attempt backfills nothing",
			statuses: []model.Service{
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusRunning, AttemptedAt: t0, LifecycleAt: t0},
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusStopping, AttemptedAt: t0, LifecycleAt: t1},
				{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusStopped, AttemptedAt: t0, LifecycleAt: t1},
			},
			wantStatus: model.StatusStopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{log: log, loader: NewLoader(), snapshot: pending}
			m.state.views = make(map[string]*serviceView)
			m.state.restarting = make(map[string]bool)
			m.applySnapshot()

			m.snapshot = &model.Snapshot{Phase: model.PhaseRunning, Resolved: true, Services: map[string]*model.Service{"id-api": &tt.statuses[0]}}
			m.applySnapshot()
			m.state.views["id-api"].StartupSampled = tt.sampledBetween

			for _, status := range tt.statuses[1:] {
				m.snapshot = &model.Snapshot{Phase: model.PhaseRunning, Resolved: true, Services: map[string]*model.Service{"id-api": &status}}
				m.applySnapshot()
			}

			view := m.state.views["id-api"]

			assert.Equal(t, tt.wantStatus, view.Status)
			assert.Equal(t, tt.wantLoader, m.loader.Has("id-api"))
			assert.Equal(t, tt.wantRestarting, m.state.restarting["id-api"])
			assert.Equal(t, tt.wantAmber, view.Timeline.Count())
			assert.Equal(t, tt.wantSampled, view.StartupSampled)
		})
	}
}

func Test_ApplySnapshot_KeepsSelectionAndFilter(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

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
	runningDB := &model.Service{ID: "id-db", Name: "db", Tier: "tier1", Status: model.StatusRunning, Process: model.Process{PID: 7}}
	failedWeb := &model.Service{ID: "id-web", Name: "web", Tier: "tier1", Status: model.StatusFailed, Error: "boom"}
	changed := &model.Snapshot{
		Phase:    model.PhaseRunning,
		Profile:  "dev",
		Resolved: true,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api, runningDB, failedWeb}}},
		Services: map[string]*model.Service{"id-api": api, "id-db": runningDB, "id-web": failedWeb},
	}

	m := Model{log: log, loader: NewLoader(), snapshot: pending}
	m.state.views = make(map[string]*serviceView)
	m.state.restarting = make(map[string]bool)
	m.applySnapshot()
	m.state.filterQuery = "b"
	m.applyFilter()
	m.state.selected = 1
	m.snapshot = changed

	cmd := m.applySnapshot()

	assert.Nil(t, cmd)
	assert.Equal(t, "b", m.state.filterQuery)
	assert.Equal(t, []string{"id-db", "id-web"}, m.state.filteredIDs)
	assert.Equal(t, 1, m.state.selected)
	assert.Equal(t, "web", m.getSelectedService().Name)
	assert.Equal(t, model.StatusFailed, m.getSelectedService().Status)
	assert.Equal(t, model.StatusRunning, m.state.views["id-db"].Status)
}

func Test_ApplySnapshot_ResolutionResetsTheView(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

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

	m := Model{log: log, loader: NewLoader(), snapshot: pending}
	m.state.views = map[string]*serviceView{"id-old": {Status: model.StatusRunning}}
	m.state.serviceIDs = []string{"id-old"}
	m.state.restarting = map[string]bool{"id-old": true}
	m.state.selected = 3
	m.state.filterQuery = "old"
	m.state.filterActive = true
	m.state.filteredIDs = []string{"id-old"}
	m.state.asideOpen = true
	m.loader.Start(loaderKeyPreflight, "preflight: scanning processes…")

	cmd := m.applySnapshot()

	assert.Nil(t, cmd)
	assert.True(t, m.state.resolved)
	assert.Equal(t, []string{"id-api", "id-db", "id-web"}, m.state.serviceIDs)
	assert.Nil(t, m.state.views["id-old"])
	assert.Empty(t, m.state.restarting)
	assert.Equal(t, 0, m.state.selected)
	assert.Empty(t, m.state.filterQuery)
	assert.False(t, m.state.filterActive)
	assert.Nil(t, m.state.filteredIDs)
	assert.False(t, m.state.asideOpen, "the aside closes when the zero-width layout cannot fit it")
	assert.Equal(t, "preflight: scanning processes…", m.loader.Message(), "a preflight announced before the resolved snapshot stays visible")
}

func Test_ApplySnapshot_BlinkAndTimelineFollowTheStatus(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	api := &model.Service{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusPending}
	pending := &model.Snapshot{
		Phase:    model.PhaseStartup,
		Profile:  "dev",
		Resolved: true,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api}}},
		Services: map[string]*model.Service{"id-api": api},
	}

	tests := []struct {
		name       string
		status     model.Status
		wantBlink  bool
		wantSlot   TimelineSlot
		wantSample int
	}{
		{
			name:       "starting blinks and samples amber",
			status:     model.StatusStarting,
			wantBlink:  true,
			wantSlot:   TimelineSlotStarting,
			wantSample: 1,
		},
		{
			name:     "running is steady and samples green",
			status:   model.StatusRunning,
			wantSlot: TimelineSlotRunning,
		},
		{
			name:       "stopping blinks and samples amber",
			status:     model.StatusStopping,
			wantBlink:  true,
			wantSlot:   TimelineSlotStarting,
			wantSample: 1,
		},
		{
			name:     "failed is steady and samples red",
			status:   model.StatusFailed,
			wantSlot: TimelineSlotFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{log: log, loader: NewLoader(), snapshot: pending}
			m.state.views = make(map[string]*serviceView)
			m.state.restarting = make(map[string]bool)
			m.applySnapshot()
			m.snapshot = &model.Snapshot{Phase: model.PhaseRunning, Resolved: true, Services: map[string]*model.Service{
				"id-api": {ID: "id-api", Name: "api", Tier: "tier1", Status: tt.status, Process: model.Process{StartedAt: t0}, AttemptedAt: t0, LifecycleAt: t0},
			}}
			m.applySnapshot()

			blinking := m.updateBlinkAnimations()
			m.sampleTimelines()

			view := m.state.views["id-api"]

			assert.Equal(t, tt.wantBlink, blinking)
			assert.Equal(t, tt.wantBlink, view.Blink.IsActive())
			assert.Equal(t, 1, view.Timeline.Count())
			assert.Equal(t, tt.wantSlot, view.Timeline.slots()[0])
			assert.Equal(t, tt.wantSample, view.StartupSampled)
		})
	}
}

func Test_HandleMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	log := slog.New(slog.DiscardHandler)

	api := &model.Service{ID: "id-api", Name: "api", Tier: "tier1", Status: model.StatusPending}
	db := &model.Service{ID: "id-db", Name: "db", Tier: "tier1", Status: model.StatusPending}
	web := &model.Service{ID: "id-web", Name: "web", Tier: "tier1", Status: model.StatusPending}
	resolved := &model.Snapshot{
		Phase:    model.PhaseStartup,
		Profile:  "dev",
		Resolved: true,
		Tiers:    []*model.Tier{{Name: "tier1", Services: []*model.Service{api, db, web}}},
		Services: map[string]*model.Service{"id-api": api, "id-db": db, "id-web": web},
	}
	empty := &model.Snapshot{}

	tests := []struct {
		name         string
		before       func() Model
		msg          contracts.Message
		wantLoader   string
		wantVersion  string
		wantResolved bool
		wantShutdown bool
	}{
		{
			name: "a changed snapshot is applied",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(resolved))

				m := Model{log: log, loader: NewLoader(), registry: mockRegistry}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)

				return m
			},
			msg:          contracts.Message{Type: contracts.EventSnapshotChanged, Data: contracts.SnapshotChanged{}},
			wantResolved: true,
		},
		{
			name: "preflight started shows the scan",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(empty))

				m := Model{log: log, loader: NewLoader(), registry: mockRegistry}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)

				return m
			},
			msg:        contracts.Message{Type: contracts.EventPreflightStarted, Data: contracts.PreflightStarted{}},
			wantLoader: "preflight: scanning processes…",
		},
		{
			name: "preflight killed names the service",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(empty))

				m := Model{log: log, loader: NewLoader(), registry: mockRegistry}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)

				return m
			},
			msg:        contracts.Message{Type: contracts.EventPreflightKilled, Data: contracts.PreflightKilled{Service: "api"}},
			wantLoader: "preflight: stopping api…",
		},
		{
			name: "preflight complete clears the scan",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(empty))

				m := Model{log: log, loader: NewLoader(), registry: mockRegistry}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)

				return m
			},
			msg: contracts.Message{Type: contracts.EventPreflightComplete, Data: contracts.PreflightComplete{}},
		},
		{
			name: "a signal starts the shutdown",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(empty))

				m := Model{log: log, loader: NewLoader(), registry: mockRegistry}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)

				return m
			},
			msg:          contracts.Message{Type: contracts.EventSignalReceived, Data: contracts.SignalReceived{Name: "SIGINT"}},
			wantLoader:   "shutting down all services…",
			wantShutdown: true,
		},
		{
			name: "an update is announced before any snapshot",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(empty))

				m := Model{log: log, loader: NewLoader(), registry: mockRegistry}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)

				return m
			},
			msg:         contracts.Message{Type: contracts.EventUpdateAvailable, Data: contracts.UpdateAvailable{Version: "v9.9.9"}},
			wantVersion: "v9.9.9",
		},
		{
			name: "an unrelated event is ignored",
			before: func() Model {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(empty))

				m := Model{log: log, loader: NewLoader(), registry: mockRegistry}
				m.state.views = make(map[string]*serviceView)
				m.state.restarting = make(map[string]bool)

				return m
			},
			msg: contracts.Message{Type: contracts.EventServiceReady, Data: contracts.ServiceReady{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			teaModel, cmd := m.Update(EventMsg(tt.msg))
			result := teaModel.(Model)

			assert.Nil(t, cmd)
			assert.Equal(t, tt.wantLoader, result.loader.Message())
			assert.Equal(t, tt.wantVersion, result.state.availableVersion)
			assert.Equal(t, tt.wantResolved, result.state.resolved)
			assert.Equal(t, tt.wantShutdown, result.state.shuttingDown)
		})
	}
}
