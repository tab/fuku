package contracts

import (
	"time"

	"fuku/internal/model"
)

// EventProfileResolved carries a ProfileResolved
const EventProfileResolved MessageType = "profile_resolved"

// ProfileResolved contains the resolved profile with its tier structure
type ProfileResolved struct {
	Profile  string
	Tiers    []model.Tier
	Duration time.Duration
}
