package contracts

import "time"

// Tier event types
const (
	EventTierStarting MessageType = "tier_starting"
	EventTierReady    MessageType = "tier_ready"
)

// TierStarting indicates a tier is beginning its startup sequence
type TierStarting struct {
	Name string
}

// TierReady indicates a tier has completed startup
type TierReady struct {
	Name         string
	Duration     time.Duration
	ServiceCount int
}
