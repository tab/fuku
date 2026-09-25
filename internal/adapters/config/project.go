package config

import (
	"sort"

	"github.com/google/uuid"

	"fuku/internal/model"
)

// Project translates adapter-owned configuration into an ordered plain model with every default applied
func Project(cfg *Config, topology *model.Topology) model.Project {
	project := model.Project{
		Services: []model.Service{},
		Profiles: map[string]model.Profile{},
	}

	project.Logging = model.Logging{Level: cfg.Logging.Level, Format: cfg.Logging.Format}
	project.Concurrency = model.Concurrency{Workers: cfg.Concurrency.Workers}
	project.Retry = model.Retry{Attempts: cfg.Retry.Attempts, Backoff: cfg.Retry.Backoff}
	project.Logs = model.Logs{Buffer: cfg.Logs.Buffer, History: cfg.Logs.History}
	project.Server = model.Server{Listen: cfg.Server.Listen, Token: cfg.Server.Auth.Token}
	project.Telemetry = model.Telemetry{Enabled: cfg.Telemetry, DSN: cfg.SentryDSN, Environment: cfg.AppEnv}
	project.Updater = model.Updater{Enabled: cfg.Updater}

	tierIndex := buildTierIndex(topology)
	project.Services = make([]model.Service, 0, len(cfg.Services))

	for name, service := range cfg.Services {
		project.Services = append(project.Services, projectService(name, service, tierIndex))
	}

	sort.Slice(project.Services, func(i, j int) bool {
		left := project.Services[i]
		right := project.Services[j]

		if tierIndex[left.Tier] != tierIndex[right.Tier] {
			return tierIndex[left.Tier] < tierIndex[right.Tier]
		}

		return left.Name < right.Name
	})

	for name, value := range cfg.Profiles {
		project.Profiles[name] = projectProfile(value)
	}

	project.Exclude = append([]string(nil), cfg.Exclude...)

	return project
}

func buildTierIndex(topology *model.Topology) map[string]int {
	order := topology.Order

	index := make(map[string]int, len(order)+1)
	for position, tier := range order {
		index[tier] = position
	}

	if _, exists := index[model.TierDefault]; !exists {
		index[model.TierDefault] = len(order)
	}

	return index
}

func projectService(name string, service *Service, tierIndex map[string]int) model.Service {
	command := service.Command
	if command == "" {
		command = DefaultServiceCommand
	}

	tier := service.Tier
	if tier == "" {
		tier = model.TierDefault
	}

	if _, exists := tierIndex[tier]; !exists {
		tier = model.TierDefault
	}

	return model.Service{
		ID:          uuid.NewString(),
		Name:        name,
		Command:     command,
		Directory:   service.Dir,
		Tier:        tier,
		Readiness:   projectReadiness(service.Readiness),
		Watch:       projectWatch(service.Watch),
		LogOutput:   projectLogOutput(service.Logs),
		Environment: projectEnvironment(service.Env),
	}
}

func projectProfile(value any) model.Profile {
	switch profile := value.(type) {
	case string:
		if profile == "*" {
			return model.Profile{All: true}
		}

		return model.Profile{Services: []string{profile}}
	case []any:
		services := make([]string, 0, len(profile))
		for _, value := range profile {
			name, _ := value.(string)
			services = append(services, name)
		}

		return model.Profile{Services: services}
	default:
		return model.Profile{}
	}
}

func projectReadiness(readiness *Readiness) *model.Readiness {
	if readiness == nil {
		return nil
	}

	return &model.Readiness{
		Type:     readiness.Type,
		Address:  readiness.Address,
		URL:      readiness.URL,
		Pattern:  readiness.Pattern,
		Timeout:  readiness.Timeout,
		Interval: readiness.Interval,
	}
}

func projectWatch(watch *Watch) *model.Watch {
	if watch == nil {
		return nil
	}

	debounce := watch.Debounce
	if debounce == 0 {
		debounce = DefaultDebounce
	}

	return &model.Watch{
		Include:  append([]string(nil), watch.Include...),
		Ignore:   append([]string(nil), watch.Ignore...),
		Shared:   append([]string(nil), watch.Shared...),
		Debounce: debounce,
	}
}

func projectLogOutput(logs *Logs) []string {
	if logs == nil || len(logs.Output) == 0 {
		return []string{"stdout", "stderr"}
	}

	return append([]string(nil), logs.Output...)
}

// projectEnvironment fills the default files for an absent list and keeps an explicit empty one
func projectEnvironment(env *Env) *model.EnvFiles {
	if env == nil || env.Files == nil {
		return &model.EnvFiles{
			Files:     []string{DefaultEnvFile, DefaultEnvFileLocal, DefaultEnvFileDevelopment, DefaultEnvFileDevelopmentLocal},
			Defaulted: true,
		}
	}

	return &model.EnvFiles{Files: append([]string{}, env.Files...)}
}
