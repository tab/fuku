package dotenv

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/config"
)

func newTestLoader(t *testing.T, cfg *config.Config) *loader {
	t.Helper()

	ctrl := gomock.NewController(t)

	return &loader{
		bus:   bus.NewMockBus(ctrl),
		cfg:   cfg,
		cache: make(map[string][]Store),
	}
}

func Test_Env_ReturnsEmptyForUnknownID(t *testing.T) {
	l := newTestLoader(t, &config.Config{})

	assert.Nil(t, l.Env("unknown-id"))
}

func Test_Env_ReturnsCopyToPreventAliasing(t *testing.T) {
	l := newTestLoader(t, &config.Config{})
	l.cache["svc-id"] = []Store{{Key: "A", Value: "1"}}

	first := l.Env("svc-id")
	first[0].Value = "mutated"

	second := l.Env("svc-id")
	assert.Equal(t, "1", second[0].Value, "Env() result must not alias the cache")
}

func Test_Reload_PopulatesCacheKeyedByID(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_NAME=hub-api\n"), 0o600))

	l := newTestLoader(t, &config.Config{})

	l.reload("uuid-123", &config.Service{Dir: dir})

	got := l.Env("uuid-123")
	assert.Equal(t, []Store{{Key: "APP_NAME", Value: "hub-api"}}, got)
}

func Test_Reload_NilEntryClearsCache(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("FOO=bar\n"), 0o600))

	l := newTestLoader(t, &config.Config{})

	l.reload("uuid-123", &config.Service{Dir: dir})
	assert.NotEmpty(t, l.Env("uuid-123"))

	l.reload("uuid-123", nil)
	assert.Nil(t, l.Env("uuid-123"), "reload with nil entry should clear the cache")
}

func Test_ServiceConfig(t *testing.T) {
	entry := &config.Service{Dir: "svc/api"}

	tests := []struct {
		name string
		cfg  *config.Config
		want *config.Service
	}{
		{
			name: "nil config",
			cfg:  nil,
			want: nil,
		},
		{
			name: "nil services map",
			cfg:  &config.Config{Services: nil},
			want: nil,
		},
		{
			name: "missing service",
			cfg:  &config.Config{Services: map[string]*config.Service{}},
			want: nil,
		},
		{
			name: "service found",
			cfg:  &config.Config{Services: map[string]*config.Service{"api": entry}},
			want: entry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newTestLoader(t, tt.cfg)
			assert.Equal(t, tt.want, l.serviceConfig("api"))
		})
	}
}

func Test_ParseService_EmptyDirReturnsNil(t *testing.T) {
	assert.Nil(t, parseService(&config.Service{}))
}

func Test_ParseService_NilEntryReturnsNil(t *testing.T) {
	assert.Nil(t, parseService(nil))
}

func Test_ParseService_ExplicitEmptyFilesDisablesLoading(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("FOO=bar\n"), 0o600))

	got := parseService(&config.Service{Dir: dir, Env: &config.Env{Files: []string{}}})
	assert.Nil(t, got)
}

func Test_ParseService_NilEnvFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.development"), []byte("APP_NAME=hub\n"), 0o600))

	got := parseService(&config.Service{Dir: dir})
	assert.Equal(t, []Store{{Key: "APP_NAME", Value: "hub"}}, got)
}

func Test_HandleProfileResolved_SeedsAllServices(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir1, ".env"), []byte("A=1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, ".env"), []byte("B=2\n"), 0o600))

	l := newTestLoader(t, &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: dir1},
			"web": {Dir: dir2},
		},
	})

	msg := bus.Message{
		Type: bus.EventProfileResolved,
		Data: bus.ProfileResolved{
			Tiers: []bus.Tier{
				{Name: "foundation", Services: []bus.Service{
					{ID: "id-api", Name: "api"},
					{ID: "id-web", Name: "web"},
				}},
			},
		},
	}

	l.handle(msg)

	assert.Equal(t, []Store{{Key: "A", Value: "1"}}, l.Env("id-api"))
	assert.Equal(t, []Store{{Key: "B", Value: "2"}}, l.Env("id-web"))
}

func Test_HandleServiceStarting_ReloadsSingleService(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("VERSION=1\n"), 0o600))

	l := newTestLoader(t, &config.Config{
		Services: map[string]*config.Service{
			"api": {Dir: dir},
		},
	})

	starting := bus.Message{
		Type: bus.EventServiceStarting,
		Data: bus.ServiceStarting{
			ServiceEvent: bus.ServiceEvent{Service: bus.Service{ID: "id-api", Name: "api"}, Tier: "foundation"},
		},
	}

	l.handle(starting)
	assert.Equal(t, []Store{{Key: "VERSION", Value: "1"}}, l.Env("id-api"))

	require.NoError(t, os.WriteFile(envPath, []byte("VERSION=2\n"), 0o600))

	l.handle(starting)
	assert.Equal(t, []Store{{Key: "VERSION", Value: "2"}}, l.Env("id-api"), "ServiceStarting should re-read .env to pick up changes")
}

func Test_Handle_IgnoresUnknownEvents(t *testing.T) {
	l := newTestLoader(t, &config.Config{})

	l.handle(bus.Message{Type: bus.EventTierReady})

	assert.Empty(t, l.cache)
}

func Test_Run_StopsOnContextCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockBus := bus.NewMockBus(ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan bus.Message)

	mockBus.EXPECT().Subscribe(ctx).Return(ch)

	l := NewLoader(Params{
		Bus:    mockBus,
		Config: &config.Config{},
	})

	done := make(chan struct{})

	go func() {
		l.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

func Test_Run_StopsOnChannelClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockBus := bus.NewMockBus(ctrl)

	ch := make(chan bus.Message)
	ctx := context.Background()

	mockBus.EXPECT().Subscribe(ctx).Return(ch)

	l := NewLoader(Params{
		Bus:    mockBus,
		Config: &config.Config{},
	})

	done := make(chan struct{})

	go func() {
		l.Run(ctx)
		close(done)
	}()

	close(ch)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after channel close")
	}
}
