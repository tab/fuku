package doctor

import (
	"context"
	"fmt"
	"strings"
)

// topologySection collects tier and profile topology checks
func topologySection(_ context.Context, env *Env) Section {
	if env.Config == nil || env.Topology == nil {
		return Section{
			Title: "Topology",
			Results: []Result{
				{
					ID:       "topology.tiers",
					Category: "topology",
					Status:   StatusIdle,
					Summary:  "skipped (config did not load)",
				},
			},
		}
	}

	return Section{
		Title: "Topology",
		Results: []Result{
			timed(func() Result { return checkTiers(env) }),
			timed(func() Result { return checkProfileResolves(env) }),
		},
	}
}

// checkTiers reports the resolved tier execution order
func checkTiers(env *Env) Result {
	topo := env.Topology

	if topo.HasDefaultOnly {
		return Result{
			ID:       "topology.tiers",
			Category: "topology",
			Status:   StatusIdle,
			Summary:  "no tiers defined (default tier only)",
		}
	}

	details := make([]Detail, 0, len(topo.Order))
	for _, tier := range topo.Order {
		details = append(details, Detail{
			Key:   tier,
			Value: fmt.Sprintf("%d services", len(topo.TierServices[tier])),
		})
	}

	return Result{
		ID:       "topology.tiers",
		Category: "topology",
		Status:   StatusOK,
		Summary:  strings.Join(topo.Order, " → "),
		Details:  details,
	}
}

// checkProfileResolves reports whether the active profile resolves to a non-empty service list
func checkProfileResolves(env *Env) Result {
	if env.ProfileErr != nil {
		return Result{
			ID:          "topology.profile",
			Category:    "topology",
			Status:      StatusFail,
			Summary:     fmt.Sprintf("profile '%s' does not resolve", env.Profile),
			Details:     []Detail{{Key: "error", Value: env.ProfileErr.Error()}},
			Remediation: "check that the profile is defined and references existing services",
		}
	}

	names := env.ProfileServices

	if len(names) == 0 {
		return Result{
			ID:       "topology.profile",
			Category: "topology",
			Status:   StatusWarn,
			Summary:  fmt.Sprintf("profile '%s' resolves to 0 services", env.Profile),
		}
	}

	return Result{
		ID:       "topology.profile",
		Category: "topology",
		Status:   StatusOK,
		Summary:  fmt.Sprintf("profile '%s' resolves to %d services", env.Profile, len(names)),
	}
}
