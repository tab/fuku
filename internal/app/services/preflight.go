package services

import (
	"context"
	"fmt"

	"fuku/internal/model"
)

// Cleaner kills whatever still runs in the service directories of a profile (the stop command)
type Cleaner struct {
	profiles  ProfileResolver
	preflight Preflight
	log       Logger
}

// NewCleaner creates the cleaner
func NewCleaner(profiles ProfileResolver, preflight Preflight, log Logger) *Cleaner {
	return &Cleaner{profiles: profiles, preflight: preflight, log: log}
}

// Cleanup resolves a profile and kills whatever still runs in its service directories
func (c *Cleaner) Cleanup(ctx context.Context, profile string) error {
	tiers, err := c.profiles.Resolve(profile)
	if err != nil {
		return fmt.Errorf("failed to resolve profile: %w", err)
	}

	services := tiers.Services()
	if len(services) == 0 {
		c.log.Warn(fmt.Sprintf("No services found for profile '%s'", profile))

		return nil
	}

	if err := c.preflight.Cleanup(ctx, serviceDirs(services)); err != nil {
		c.log.Warn("Preflight cleanup failed during stop", "error", err)
	}

	return nil
}

// serviceDirs maps the service names to their configured directories, the scope of a preflight cleanup
func serviceDirs(services []*model.Service) map[string]string {
	dirs := make(map[string]string, len(services))

	for _, service := range services {
		dirs[service.Name] = service.Directory
	}

	return dirs
}
