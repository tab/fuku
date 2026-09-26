package doctor

import (
	"context"
	"sort"

	"fuku/internal/model"
)

// Options controls a doctor run (Version is the running fuku version the report is stamped with)
type Options struct {
	Profile        string
	ExplicitConfig bool
	Fingerprint    string
	Version        string
}

// Profiles resolves a profile of the loaded project into its tiers
type Profiles interface {
	Resolve(profile string) (model.Tiers, error)
}

// Runner executes the doctor checks over the loaded config and the observers that see the machine
type Runner struct {
	options     Options
	config      model.Config
	environment Environment
	filesystem  Filesystem
	profiles    Profiles
	runtime     Runtime
}

// NewRunner creates a doctor runner over the loaded config
func NewRunner(options Options, config model.Config, environment Environment, filesystem Filesystem, profiles Profiles, runtime Runtime) *Runner {
	return &Runner{
		options:     options,
		config:      config,
		environment: environment,
		filesystem:  filesystem,
		profiles:    profiles,
		runtime:     runtime,
	}
}

// Run executes all doctor checks and returns the report
func (r *Runner) Run(ctx context.Context) *model.Report {
	st := r.load()

	report := newReport(r.options.Version)
	report.Sections = []model.Section{
		r.environmentSection(),
		configSection(st),
		r.servicesSection(st),
		topologySection(st),
		r.runtimeSection(ctx, st),
	}

	return report
}

// state is the loaded config and the resolved profile every check reads
type state struct {
	Options
	model.Config
	services   []*model.Service
	profileErr error
}

// loaded reports whether the config was read, parsed and validated
func (s *state) loaded() bool {
	return s.Error == nil
}

// load unpacks the loaded config and resolves the active profile when the config loaded
func (r *Runner) load() *state {
	st := &state{Options: r.options, Config: r.config}

	if st.loaded() {
		st.services, st.profileErr = resolveProfileServices(r.profiles, st.Profile)
	}

	return st
}

// resolveProfileServices returns the services in the active profile sorted by name
func resolveProfileServices(profiles Profiles, profile string) ([]*model.Service, error) {
	tiers, err := profiles.Resolve(profile)
	if err != nil {
		return nil, err
	}

	services := tiers.Services()
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	return services, nil
}
