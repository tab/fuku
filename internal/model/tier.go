package model

// Tier is a group of services that start together, with its readiness
type Tier struct {
	ID       string
	Name     string
	Ready    bool
	Services []*Service
}
