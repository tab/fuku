package config

import (
	"sort"

	"go.yaml.in/yaml/v3"
)

// Topology represents the derived tier ordering and grouping metadata
type Topology struct {
	Order          []string
	TierServices   map[string][]string
	HasDefaultOnly bool
}

// DefaultTopology returns the default topology
func DefaultTopology() *Topology {
	return &Topology{
		Order:          []string{},
		TierServices:   make(map[string][]string),
		HasDefaultOnly: true,
	}
}

// parseTierOrder reads YAML config bytes and extracts tier ordering
func parseTierOrder(data []byte) (*Topology, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	topology := &Topology{
		Order:        []string{},
		TierServices: make(map[string][]string),
	}

	tierSeen := make(map[string]bool)
	hasDefaultServices := false

	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return topology, nil
	}

	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return topology, nil
	}

	defaultTier := ""

	for i := 0; i < len(doc.Content); i += 2 {
		key := doc.Content[i]
		value := doc.Content[i+1]

		if key.Value == "defaults" && value.Kind == yaml.MappingNode {
			defaults := flattenMergeKeys(value)

			for j := 0; j < len(defaults.Content); j += 2 {
				fieldKey := defaults.Content[j]
				fieldValue := defaults.Content[j+1]

				if fieldKey.Value == "tier" {
					defaultTier = normalizeTier(fieldValue.Value)
					break
				}
			}
		}
	}

	for i := 0; i < len(doc.Content); i += 2 {
		key := doc.Content[i]
		value := doc.Content[i+1]

		if key.Value != "services" || value.Kind != yaml.MappingNode {
			continue
		}

		for j := 0; j < len(value.Content); j += 2 {
			serviceName := value.Content[j].Value
			serviceNode := value.Content[j+1]

			if serviceNode.Kind != yaml.MappingNode {
				continue
			}

			tier := ""
			flatService := flattenMergeKeys(serviceNode)

			for k := 0; k < len(flatService.Content); k += 2 {
				fieldKey := flatService.Content[k]
				fieldValue := flatService.Content[k+1]

				if fieldKey.Value == "tier" {
					tier = normalizeTier(fieldValue.Value)
					break
				}
			}

			switch {
			case tier != "":
				// tier already set from service config
			case defaultTier != "":
				tier = defaultTier
			default:
				tier = Default
				hasDefaultServices = true
			}

			if tier != Default && !tierSeen[tier] {
				tierSeen[tier] = true
				topology.Order = append(topology.Order, tier)
			}

			topology.TierServices[tier] = append(topology.TierServices[tier], serviceName)
		}
	}

	if hasDefaultServices {
		topology.Order = append(topology.Order, Default)
	}

	for tier := range topology.TierServices {
		sort.Strings(topology.TierServices[tier])
	}

	topology.HasDefaultOnly = len(topology.Order) == 0 || (len(topology.Order) == 1 && topology.Order[0] == Default)

	return topology, nil
}
