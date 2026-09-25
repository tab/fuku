package doctor

import (
	"fmt"
	"strings"

	"fuku/internal/model"
)

// topologySection collects tier and profile topology checks
func topologySection(st *state) model.Section {
	if !st.loaded() {
		return model.Section{
			Title: "Topology",
			Note:  "skipped (config did not load)",
			Results: []model.Result{
				skipped(model.CheckTopologyTiers, model.CategoryTopology, "config did not load"),
				skipped(model.CheckTopologyProfile, model.CategoryTopology, "config did not load"),
			},
		}
	}

	return model.Section{
		Title: "Topology",
		Results: []model.Result{
			timed(func() model.Result { return checkTiers(st) }),
			timed(func() model.Result { return checkProfileResolves(st) }),
		},
	}
}

// checkTiers reports the resolved tier execution order
func checkTiers(st *state) model.Result {
	topo := st.Topology

	if topo.DefaultOnly() {
		return model.Result{
			ID:       model.CheckTopologyTiers,
			Category: model.CategoryTopology,
			Severity: model.SeverityIdle,
			Summary:  "no tiers defined (default tier only)",
		}
	}

	details := make([]model.Detail, 0, len(topo.Order))
	for _, tier := range topo.Order {
		details = append(details, model.Detail{
			Key:   tier,
			Value: fmt.Sprintf("%d services", len(topo.TierServices[tier])),
		})
	}

	return model.Result{
		ID:       model.CheckTopologyTiers,
		Category: model.CategoryTopology,
		Severity: model.SeverityOK,
		Summary:  strings.Join(topo.Order, " → "),
		Details:  details,
	}
}

// checkProfileResolves reports whether the active profile resolves to a non-empty service list
func checkProfileResolves(st *state) model.Result {
	if st.profileErr != nil {
		return model.Result{
			ID:          model.CheckTopologyProfile,
			Category:    model.CategoryTopology,
			Severity:    model.SeverityFail,
			Summary:     fmt.Sprintf("profile '%s' does not resolve", st.Profile),
			Details:     []model.Detail{{Key: "error", Value: st.profileErr.Error()}},
			Remediation: "check that the profile is defined and references existing services",
		}
	}

	if len(st.services) == 0 {
		return model.Result{
			ID:       model.CheckTopologyProfile,
			Category: model.CategoryTopology,
			Severity: model.SeverityWarn,
			Summary:  fmt.Sprintf("profile '%s' resolves to 0 services", st.Profile),
		}
	}

	return model.Result{
		ID:       model.CheckTopologyProfile,
		Category: model.CategoryTopology,
		Severity: model.SeverityOK,
		Summary:  fmt.Sprintf("profile '%s' resolves to %d services", st.Profile, len(st.services)),
	}
}
