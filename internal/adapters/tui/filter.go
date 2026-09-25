package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"fuku/internal/model"
)

// normalizeQuery trims leading/trailing dashes, underscores, and spaces, then lowercases
func normalizeQuery(raw string) string {
	return strings.ToLower(strings.Trim(raw, "-_ "))
}

// nameMatches reports whether the service name contains the normalized query, ignoring case
func nameMatches(service *model.Service, query string) bool {
	return strings.Contains(strings.ToLower(service.Name), query)
}

// filterServiceIDs returns the subset of allIDs whose service names match the query, preserving order
func filterServiceIDs(query string, allIDs []string, services map[string]*model.Service) []string {
	q := normalizeQuery(query)
	if q == "" {
		return allIDs
	}

	result := make([]string, 0, len(allIDs))

	for _, id := range allIDs {
		if nameMatches(services[id], q) {
			result = append(result, id)
		}
	}

	return result
}

// filterTiers returns tiers with only matching services, omitting tiers that have no matches
func filterTiers(query string, tiers []*model.Tier) []*model.Tier {
	q := normalizeQuery(query)
	if q == "" {
		return tiers
	}

	result := make([]*model.Tier, 0, len(tiers))

	for _, tier := range tiers {
		matched := make([]*model.Service, 0, len(tier.Services))

		for _, svc := range tier.Services {
			if nameMatches(svc, q) {
				matched = append(matched, svc)
			}
		}

		if len(matched) > 0 {
			result = append(result, &model.Tier{ID: tier.ID, Name: tier.Name, Ready: tier.Ready, Services: matched})
		}
	}

	return result
}

// handleFilterKey enters filter input mode
func (m Model) handleFilterKey() (Model, tea.Cmd) {
	m.state.filterActive = true

	ids := m.activeServiceIDs()
	if m.state.selected >= 0 && m.state.selected < len(ids) {
		m.state.preFilterSelectedID = ids[m.state.selected]
	}

	return m, nil
}

// clearFilter exits filter mode, restores the full list, and preserves selection on the same service
func (m *Model) clearFilter() {
	var previousID string

	if m.state.filteredIDs != nil && m.state.selected >= 0 && m.state.selected < len(m.state.filteredIDs) {
		previousID = m.state.filteredIDs[m.state.selected]
	}

	if previousID == "" && m.state.filteredIDs == nil && m.state.selected >= 0 && m.state.selected < len(m.state.serviceIDs) {
		previousID = m.state.serviceIDs[m.state.selected]
	}

	if previousID == "" {
		previousID = m.state.lastFilteredSelectedID
	}

	if previousID == "" {
		previousID = m.state.preFilterSelectedID
	}

	m.state.filterActive = false
	m.state.filterQuery = ""
	m.state.filteredIDs = nil
	m.state.preFilterSelectedID = ""
	m.state.lastFilteredSelectedID = ""

	m.state.selected = 0

	for i, id := range m.state.serviceIDs {
		if id == previousID {
			m.state.selected = i

			break
		}
	}

	m.refreshSelection()
}

// handleFilterInput processes key events while in filter input mode
func (m Model) handleFilterInput(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEscape:
		m.clearFilter()

		return m, nil

	case tea.KeyEnter:
		m.state.filterActive = false

		return m, nil

	case tea.KeyBackspace:
		if len(m.state.filterQuery) > 0 {
			runes := []rune(m.state.filterQuery)
			m.state.filterQuery = string(runes[:len(runes)-1])
			m.applyFilter()
		}

		return m, nil

	case tea.KeyUp:
		return m.handleMoveKey(msg, -1)

	case tea.KeyDown:
		return m.handleMoveKey(msg, 1)

	default:
		if msg.Text != "" {
			m.state.filterQuery += msg.Text
			m.applyFilter()
		}

		return m, nil
	}
}

// applyFilter recomputes filteredIDs from the current query and adjusts selection
func (m *Model) applyFilter() {
	var previousID string

	ids := m.state.filteredIDs
	if ids == nil {
		ids = m.state.serviceIDs
	}

	if m.state.selected >= 0 && m.state.selected < len(ids) {
		previousID = ids[m.state.selected]
	}

	if previousID == "" {
		previousID = m.state.lastFilteredSelectedID
	}

	m.state.filteredIDs = filterServiceIDs(m.state.filterQuery, m.state.serviceIDs, m.snapshot.Services)

	m.state.selected = 0

	for i, id := range m.state.filteredIDs {
		if id == previousID {
			m.state.selected = i

			break
		}
	}

	if previousID != "" {
		m.state.lastFilteredSelectedID = previousID
	}

	m.refreshSelection()
}
