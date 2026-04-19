package services

import "strings"

// normalizeQuery trims leading/trailing dashes, underscores, and spaces, then lowercases
func normalizeQuery(raw string) string {
	return strings.ToLower(strings.Trim(raw, "-_ "))
}

// filterServiceIDs returns the subset of allIDs whose service names match the query, preserving order
func filterServiceIDs(query string, allIDs []string, services map[string]*ServiceState) []string {
	q := normalizeQuery(query)
	if q == "" {
		return allIDs
	}

	result := make([]string, 0, len(allIDs))

	for _, id := range allIDs {
		svc, exists := services[id]
		if !exists {
			continue
		}

		if strings.Contains(strings.ToLower(svc.Name), q) {
			result = append(result, id)
		}
	}

	return result
}

// filterTiers returns tiers with only matching services, omitting tiers that have no matches
func filterTiers(query string, tiers []Tier, services map[string]*ServiceState) []Tier {
	q := normalizeQuery(query)
	if q == "" {
		return tiers
	}

	result := make([]Tier, 0, len(tiers))

	for _, tier := range tiers {
		matched := make([]string, 0, len(tier.Services))

		for _, id := range tier.Services {
			svc, exists := services[id]
			if !exists {
				continue
			}

			if strings.Contains(strings.ToLower(svc.Name), q) {
				matched = append(matched, id)
			}
		}

		if len(matched) > 0 {
			result = append(result, Tier{
				Name:     tier.Name,
				Services: matched,
				Ready:    tier.Ready,
			})
		}
	}

	return result
}
