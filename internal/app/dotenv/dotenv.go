package dotenv

import (
	"context"
	"sync"

	"go.uber.org/fx"

	"fuku/internal/app/bus"
	"fuku/internal/config"
)

// Default .env file names merged when a service does not override env.files
const (
	envFile                 = ".env"
	envLocalFile            = ".env.local"
	envDevelopmentFile      = ".env.development"
	envDevelopmentLocalFile = ".env.development.local"
)

// Store is a single key/value entry parsed from a service's .env files
type Store struct {
	Key   string
	Value string
}

// Loader maintains a per-service cache of merged .env file entries for display in the UI's env tab; values are never injected into the service process environment
type Loader interface {
	Run(ctx context.Context)
	Env(id string) []Store
}

// Params contains the dependencies needed to construct a Loader
type Params struct {
	fx.In

	Bus    bus.Bus
	Config *config.Config
}

type loader struct {
	bus bus.Bus
	cfg *config.Config

	mu    sync.RWMutex
	cache map[string][]Store
}

// NewLoader creates a Loader that subscribes to lifecycle events and caches merged .env entries per service ID
func NewLoader(p Params) Loader {
	return &loader{
		bus:   p.Bus,
		cfg:   p.Config,
		cache: make(map[string][]Store),
	}
}

// Run subscribes to the bus and updates the per-service env cache on lifecycle events
func (l *loader) Run(ctx context.Context) {
	ch := l.bus.Subscribe(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			l.handle(msg)
		}
	}
}

// Env returns the merged .env entries cached for the given service ID
func (l *loader) Env(id string) []Store {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entries := l.cache[id]
	if len(entries) == 0 {
		return nil
	}

	out := make([]Store, len(entries))
	copy(out, entries)

	return out
}

func (l *loader) handle(msg bus.Message) {
	//nolint:exhaustive // only lifecycle events trigger env reload
	switch msg.Type {
	case bus.EventProfileResolved:
		l.handleProfileResolved(msg)
	case bus.EventServiceStarting:
		l.handleServiceStarting(msg)
	}
}

func (l *loader) handleProfileResolved(msg bus.Message) {
	data, ok := msg.Data.(bus.ProfileResolved)
	if !ok {
		return
	}

	for _, tier := range data.Tiers {
		for _, svc := range tier.Services {
			l.reload(svc.ID, l.serviceConfig(svc.Name))
		}
	}
}

func (l *loader) handleServiceStarting(msg bus.Message) {
	data, ok := msg.Data.(bus.ServiceStarting)
	if !ok {
		return
	}

	l.reload(data.Service.ID, l.serviceConfig(data.Service.Name))
}

// serviceConfig translates a service name to its config entry at the package boundary; *config.Config.Services is YAML-keyed by Name, so the translation happens here and nowhere else in the package
func (l *loader) serviceConfig(name string) *config.Service {
	if l.cfg == nil || l.cfg.Services == nil {
		return nil
	}

	return l.cfg.Services[name]
}

func (l *loader) reload(id string, entry *config.Service) {
	entries := parseService(entry)

	l.mu.Lock()
	l.cache[id] = entries
	l.mu.Unlock()
}

func parseService(entry *config.Service) []Store {
	if entry == nil || entry.Dir == "" {
		return nil
	}

	return mergeFiles(entry.Dir, resolveFiles(entry))
}

// resolveFiles returns the configured env.files list, falling back to defaults when unset
func resolveFiles(entry *config.Service) []string {
	if entry.Env != nil && entry.Env.Files != nil {
		return entry.Env.Files
	}

	return []string{envFile, envLocalFile, envDevelopmentFile, envDevelopmentLocalFile}
}
