package discovery

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

// discovery implements the Discovery interface
type discovery struct {
	cfg      *config.Config
	topology *config.Topology
}

// NewDiscovery creates a new service discovery instance
func NewDiscovery(cfg *config.Config, topology *config.Topology) Discovery {
	return &discovery{
		cfg:      cfg,
		topology: topology,
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

	if len(services) == 0 {
		return []Tier{}, nil
	}

	tiers := d.groupServicesByTier(services)

	return tiers, nil
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
			services := make([]string, 0, len(d.cfg.Services))
			for name := range d.cfg.Services {
				services = append(services, name)
			}

			return services, nil
		}

		return []string{v}, nil
	case []interface{}:
		services := make([]string, 0, len(v))

		for _, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%w: profile '%s' contains non-string entry", errors.ErrUnsupportedProfileFormat, profile)
			}

			services = append(services, str)
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

	tierIndexMap := d.buildTierIndexMap()

	sort.SliceStable(result, func(i, j int) bool {
		svcI := d.cfg.Services[result[i]]
		svcJ := d.cfg.Services[result[j]]

		tierI := svcI.Tier
		if tierI == "" {
			tierI = config.Default
		}

		tierJ := svcJ.Tier
		if tierJ == "" {
			tierJ = config.Default
		}

		return d.getTierIndex(tierI, tierIndexMap) < d.getTierIndex(tierJ, tierIndexMap)
	})

	return result, nil
}

// buildTierIndexMap creates a map of tier names to their index in the tier order
func (d *discovery) buildTierIndexMap() map[string]int {
	tierIndexMap := make(map[string]int)

	for i, tier := range d.topology.Order {
		tierIndexMap[tier] = i
	}

	if _, exists := tierIndexMap[config.Default]; !exists {
		tierIndexMap[config.Default] = len(d.topology.Order)
	}

	return tierIndexMap
}

// getTierIndex returns the index for a tier, normalizing unknown tiers to default
func (d *discovery) getTierIndex(tierName string, tierIndexMap map[string]int) int {
	if idx, exists := tierIndexMap[tierName]; exists {
		return idx
	}

	return tierIndexMap[config.Default]
}

// groupServicesByTier groups services by their tier order
func (d *discovery) groupServicesByTier(services []string) []Tier {
	tierIndexMap := d.buildTierIndexMap()
	tiers := make(map[int]*Tier)

	for _, name := range services {
		srv := d.cfg.Services[name]
		tierName := srv.Tier

		if tierName == "" {
			tierName = config.Default
		}

		if _, exists := tierIndexMap[tierName]; !exists {
			tierName = config.Default
		}

		tierIndex := d.getTierIndex(tierName, tierIndexMap)

		if tiers[tierIndex] == nil {
			tiers[tierIndex] = &Tier{Name: tierName, Services: []string{}}
		}

		tiers[tierIndex].Services = append(tiers[tierIndex].Services, name)
	}

	tierOrders := make([]int, 0, len(tiers))
	for order := range tiers {
		tierOrders = append(tierOrders, order)
	}

	sort.Ints(tierOrders)

	result := make([]Tier, 0, len(tierOrders))
	for _, order := range tierOrders {
		sort.Strings(tiers[order].Services)
		result = append(result, *tiers[order])
	}

	return result
}
