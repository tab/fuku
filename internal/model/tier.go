package model

// Tier is a group of services that start together, with its readiness
type Tier struct {
	ID       string
	Name     string
	Ready    bool
	Services []*Service
}

// Tiers is a resolved profile: its tiers in startup order
type Tiers []Tier

// Services lists the services of every tier in startup order
func (t Tiers) Services() []*Service {
	var services []*Service

	for _, tier := range t {
		services = append(services, tier.Services...)
	}

	return services
}
