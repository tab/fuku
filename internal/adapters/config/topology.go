package config

import (
	"sort"

	"go.yaml.in/yaml/v3"

	"fuku/internal/model"
)

// defaultTopology returns the topology of a project without tiers
func defaultTopology() *model.Topology {
	return &model.Topology{
		Order:        []string{},
		TierServices: make(map[string][]string),
	}
}

// parseTierOrder reads YAML config bytes and extracts tier ordering
func parseTierOrder(data []byte) (*model.Topology, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	topology := &model.Topology{
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

		if key.Value != keyDefaults || value.Kind != yaml.MappingNode {
			continue
		}

		defaultTier = tierOf(value)
	}

	for i := 0; i < len(doc.Content); i += 2 {
		key := doc.Content[i]
		value := doc.Content[i+1]

		if key.Value != keyServices || value.Kind != yaml.MappingNode {
			continue
		}

		for j := 0; j < len(value.Content); j += 2 {
			serviceName := value.Content[j].Value
			serviceNode := value.Content[j+1]

			if serviceNode.Kind != yaml.MappingNode {
				continue
			}

			tier := tierOf(serviceNode)

			if tier == "" {
				tier = defaultTier
			}

			if tier == "" {
				tier = model.TierDefault
				hasDefaultServices = true
			}

			if tier != model.TierDefault && !tierSeen[tier] {
				tierSeen[tier] = true
				topology.Order = append(topology.Order, tier)
			}

			topology.TierServices[tier] = append(topology.TierServices[tier], serviceName)
		}
	}

	if hasDefaultServices {
		topology.Order = append(topology.Order, model.TierDefault)
	}

	for tier := range topology.TierServices {
		sort.Strings(topology.TierServices[tier])
	}

	return topology, nil
}

// tierOf returns the normalized tier a mapping node declares, merge keys included (empty when it declares none)
func tierOf(node *yaml.Node) string {
	flat := flattenMergeKeys(node)

	for i := 0; i < len(flat.Content); i += 2 {
		if flat.Content[i].Value == keyTier {
			return normalizeTier(flat.Content[i+1].Value)
		}
	}

	return ""
}
