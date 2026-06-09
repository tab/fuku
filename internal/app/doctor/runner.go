package doctor

import (
	"context"
	"runtime"
	"time"

	"fuku/internal/config"
)

// Options controls a doctor run
type Options struct {
	Profile    string
	ConfigPath string
}

// Env holds shared loaded state passed to each check
type Env struct {
	Profile         string
	ConfigPath      string
	ExplicitConfig  bool
	OverridePath    string
	Config          *config.Config
	Topology        *config.Topology
	LoadErr         error
	ProfileServices []string
	ProfileErr      error
}

// Run executes all doctor checks and returns the report
func Run(ctx context.Context, opts Options) *Report {
	env := loadEnv(opts)

	report := &Report{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC(),
		FukuVersion:   config.Version,
		Platform:      runtime.GOOS + "-" + runtime.GOARCH,
	}

	report.Sections = []Section{
		environmentSection(ctx, env),
		configSection(ctx, env),
		servicesSection(ctx, env),
		topologySection(ctx, env),
		runtimeSection(ctx, env),
	}

	return report
}

// loadEnv resolves config paths and attempts to load the config without panicking on failure
func loadEnv(opts Options) *Env {
	env := &Env{Profile: opts.Profile, ExplicitConfig: opts.ConfigPath != ""}

	env.ConfigPath, _ = config.ResolveConfigPath(opts.ConfigPath)
	env.OverridePath, _ = config.ResolveOverridePath(env.ConfigPath)

	cfg, topo, err := config.LoadPath(opts.ConfigPath)
	env.Config = cfg
	env.Topology = topo
	env.LoadErr = err

	if cfg != nil && topo != nil {
		env.ProfileServices, env.ProfileErr = resolveProfileServices(env)
	}

	return env
}
