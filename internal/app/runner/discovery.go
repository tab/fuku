package runner

import (
	"fmt"
	"sort"

	"fuku/internal/app/errors"
	"fuku/internal/config"
)

// Tier represents a group of services in the same deployment tier
type Tier struct {
	Name     string
	Services []string
}

// Discovery handles service ordering and tier grouping
type Discovery interface {
	Resolve(profile string) ([]Tier, error)
}

type discovery struct {
	cfg *config.Config
}

// NewDiscovery creates a new service discovery instance
func NewDiscovery(cfg *config.Config) Discovery {
	return &discovery{
		cfg: cfg,
	}
}

// Resolve returns services grouped by tier for a given profile
func (d *discovery) Resolve(profile string) ([]Tier, error) {
	serviceNames, err := d.getServicesForProfile(profile)
	if err != nil {
		return nil, err
	}

	services, err := d.resolveServiceOrder(serviceNames)
	if err != nil {
		return nil, err
	}

	return d.groupServicesByTier(services), nil
}

// getServicesForProfile returns the list of services for a given profile
func (d *discovery) getServicesForProfile(profile string) ([]string, error) {
	profileConfig, exists := d.cfg.Profiles[profile]

	if !exists {
		return nil, fmt.Errorf("%w: %s", errors.ErrProfileNotFound, profile)
	}

	switch v := profileConfig.(type) {
	case string:
		if v == "*" {
			var allServices []string
			for name := range d.cfg.Services {
				allServices = append(allServices, name)
			}
			return allServices, nil
		}
		return []string{v}, nil
	case []interface{}:
		var services []string
		for _, item := range v {
			if str, ok := item.(string); ok {
				services = append(services, str)
			}
		}
		return services, nil
	default:
		return nil, fmt.Errorf("%w: %s", errors.ErrUnsupportedProfileFormat, profile)
	}
}

// resolveServiceOrder validates, deduplicates, and orders services by tier
func (d *discovery) resolveServiceOrder(serviceNames []string) ([]string, error) {
	for _, serviceName := range serviceNames {
		if _, exists := d.cfg.Services[serviceName]; !exists {
			return nil, fmt.Errorf("%w: '%s'", errors.ErrServiceNotFound, serviceName)
		}
	}

	seen := make(map[string]bool)
	result := make([]string, 0, len(serviceNames))
	for _, serviceName := range serviceNames {
		if !seen[serviceName] {
			seen[serviceName] = true
			result = append(result, serviceName)
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		svcI := d.cfg.Services[result[i]]
		svcJ := d.cfg.Services[result[j]]
		return d.getTierOrder(svcI.Tier) < d.getTierOrder(svcJ.Tier)
	})

	return result, nil
}

// groupServicesByTier groups services by their tier order
func (d *discovery) groupServicesByTier(services []string) []Tier {
	tiers := make(map[int]*Tier)

	for _, name := range services {
		srv := d.cfg.Services[name]
		tierOrder := d.getTierOrder(srv.Tier)

		if tiers[tierOrder] == nil {
			tierName := srv.Tier
			if tierName == "" {
				tierName = config.Default
			}
			tiers[tierOrder] = &Tier{Name: tierName, Services: []string{}}
		}

		tiers[tierOrder].Services = append(tiers[tierOrder].Services, name)
	}

	tierOrders := make([]int, 0, len(tiers))
	for order := range tiers {
		tierOrders = append(tierOrders, order)
	}
	sort.Ints(tierOrders)

	result := make([]Tier, 0, len(tierOrders))
	for _, order := range tierOrders {
		result = append(result, *tiers[order])
	}

	return result
}

func (d *discovery) getTierOrder(tier string) int {
	switch tier {
	case config.Foundation:
		return 0
	case config.Platform:
		return 1
	case config.Edge:
		return 2
	default:
		return 3
	}
}
