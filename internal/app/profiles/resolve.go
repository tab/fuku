package profiles

import (
	"fmt"

	"github.com/google/uuid"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Resolver expands configured profiles into ordered tiers while preserving service identities and configuration
type Resolver struct {
	project model.Project
}

// NewResolver creates a profile resolver from the plain project model
func NewResolver(project model.Project) *Resolver {
	return &Resolver{project: project}
}

// Resolve returns the profile's services grouped by tier
func (r *Resolver) Resolve(profile string) (model.Tiers, error) {
	configured, exists := r.project.Profiles[profile]
	if !exists {
		return nil, fmt.Errorf("%w: %s", contracts.ErrProfileNotFound, profile)
	}

	names := configured.Services
	if configured.All {
		names = make([]string, 0, len(r.project.Services))
		for _, service := range r.project.Services {
			names = append(names, service.Name)
		}
	}

	excluded := make(map[string]bool, len(r.project.Exclude))
	for _, name := range r.project.Exclude {
		excluded[name] = true
	}

	selected := make(map[string]bool, len(names))
	for _, name := range names {
		if excluded[name] {
			continue
		}

		_, exists := r.project.Service(name)
		if !exists {
			return nil, fmt.Errorf("%w: '%s'", contracts.ErrServiceNotFound, name)
		}

		selected[name] = true
	}

	tiers := make(model.Tiers, 0)

	for _, service := range r.project.Services {
		if !selected[service.Name] {
			continue
		}

		if len(tiers) == 0 || tiers[len(tiers)-1].Name != service.Tier {
			tiers = append(tiers, model.Tier{ID: uuid.NewString(), Name: service.Tier, Services: []*model.Service{}})
		}

		last := len(tiers) - 1
		tiers[last].Services = append(tiers[last].Services, &service)
	}

	return tiers, nil
}
