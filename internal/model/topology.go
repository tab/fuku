package model

// TierDefault is the tier of a service that declares none
const TierDefault = "default"

// Topology is the tier order and grouping derived from the declaration order of the config file
type Topology struct {
	Order        []string
	TierServices map[string][]string
}

// DefaultOnly reports whether the default tier is the only tier (an empty order counts)
func (t Topology) DefaultOnly() bool {
	return len(t.Order) == 0 || (len(t.Order) == 1 && t.Order[0] == TierDefault)
}
