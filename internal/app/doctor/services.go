package doctor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"fuku/internal/app/discovery"
	"fuku/internal/config"
)

// servicesSection collects per-service filesystem and field checks
func servicesSection(_ context.Context, env *Env) Section {
	section := Section{
		Title: "Services",
	}

	if env.Config == nil {
		section.Note = "skipped (config did not load)"
		section.Results = skippedServiceResults("config did not load")

		return section
	}

	// The profile resolution failure is reported authoritatively by topology.profile,
	// so here we only skip the per-service checks that depend on it.
	if env.ProfileErr != nil {
		section.Note = "skipped (profile did not resolve)"
		section.Results = skippedServiceResults("profile did not resolve")

		return section
	}

	names := env.ProfileServices

	section.Note = fmt.Sprintf("active profile: %s · %d services", env.Profile, len(names))
	section.Results = []Result{
		timed(func() Result { return checkServiceDirectories(env, names) }),
		timed(func() Result { return checkServiceDotenv(env, names) }),
		timed(func() Result { return checkServiceReadiness(env, names) }),
	}

	return section
}

// checkServiceDirectories verifies that each service.Dir exists on disk
func checkServiceDirectories(env *Env, names []string) Result {
	var missing []string

	details := make([]Detail, 0, len(names))

	for _, name := range names {
		svc := env.Config.Services[name]
		dir := serviceDirAbs(svc)

		if dirExists(dir) {
			details = append(details, Detail{Key: name, Value: dir})
			continue
		}

		missing = append(missing, name)
		details = append(details, Detail{Key: name, Value: dir + " (MISSING)"})
	}

	if len(missing) == 0 {
		return Result{
			ID:       "services.directories",
			Category: "services",
			Status:   StatusOK,
			Summary:  fmt.Sprintf("%d of %d directories present", len(names), len(names)),
			Details:  details,
		}
	}

	return Result{
		ID:          "services.directories",
		Category:    "services",
		Status:      StatusWarn,
		Summary:     fmt.Sprintf("%d of %d directories missing", len(missing), len(names)),
		Details:     details,
		Remediation: "create the missing directories or fix `dir:` paths in fuku.yaml",
	}
}

// checkServiceDotenv verifies that each referenced .env file exists and is readable
func checkServiceDotenv(env *Env, names []string) Result {
	var missing []string

	total := 0
	details := []Detail{}

	for _, name := range names {
		svc := env.Config.Services[name]
		if svc.Env == nil || len(svc.Env.Files) == 0 {
			continue
		}

		dir := serviceDirAbs(svc)

		for _, file := range svc.Env.Files {
			total++

			path := filepath.Join(dir, file)
			if fileExists(path) {
				continue
			}

			label := fmt.Sprintf("%s/%s", name, file)
			missing = append(missing, label)
			details = append(details, Detail{Key: label, Value: "MISSING"})
		}
	}

	if total == 0 {
		return Result{
			ID:       "services.dotenv",
			Category: "services",
			Status:   StatusIdle,
			Summary:  "no .env files referenced",
		}
	}

	if len(missing) == 0 {
		return Result{
			ID:       "services.dotenv",
			Category: "services",
			Status:   StatusOK,
			Summary:  fmt.Sprintf("%d files referenced, all readable", total),
		}
	}

	return Result{
		ID:          "services.dotenv",
		Category:    "services",
		Status:      StatusWarn,
		Summary:     fmt.Sprintf("%d of %d referenced .env files missing", len(missing), total),
		Details:     details,
		Remediation: "create the missing .env files or update `env.files` in fuku.yaml",
	}
}

// checkServiceReadiness verifies probe fields parse as URL, regex, or host:port
func checkServiceReadiness(env *Env, names []string) Result {
	var (
		http, tcp, log int
		issues         []Detail
	)

	for _, name := range names {
		svc := env.Config.Services[name]
		if svc.Readiness == nil {
			continue
		}

		r := svc.Readiness

		switch r.Type {
		case config.TypeHTTP:
			http++

			if err := validateHTTPURL(r.URL); err != nil {
				issues = append(issues, Detail{Key: name + " url", Value: err.Error()})
			}
		case config.TypeTCP:
			tcp++

			if _, _, err := net.SplitHostPort(r.Address); err != nil {
				issues = append(issues, Detail{Key: name + " address", Value: err.Error()})
			}
		case config.TypeLog:
			log++

			if _, err := regexp.Compile(r.Pattern); err != nil {
				issues = append(issues, Detail{Key: name + " pattern", Value: err.Error()})
			}
		}
	}

	total := http + tcp + log
	if total == 0 {
		return Result{
			ID:       "services.readiness",
			Category: "services",
			Status:   StatusIdle,
			Summary:  "no readiness probes defined",
		}
	}

	if len(issues) > 0 {
		return Result{
			ID:          "services.readiness",
			Category:    "services",
			Status:      StatusFail,
			Summary:     fmt.Sprintf("%d probe field(s) failed to parse", len(issues)),
			Details:     issues,
			Remediation: "fix the malformed readiness fields in fuku.yaml",
		}
	}

	return Result{
		ID:       "services.readiness",
		Category: "services",
		Status:   StatusOK,
		Summary:  fmt.Sprintf("%d probes parse (http=%d tcp=%d log=%d)", total, http, tcp, log),
	}
}

// validateHTTPURL returns an error unless u parses as an absolute http(s) URL with a host
func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("scheme must be http or https")
	}

	if u.Host == "" {
		return errors.New("missing host")
	}

	return nil
}

// resolveProfileServices returns the sorted list of services in the active profile
func resolveProfileServices(env *Env) ([]string, error) {
	disc := discovery.NewDiscovery(env.Config, env.Topology)

	tiers, err := disc.Resolve(env.Profile)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, tier := range tiers {
		names = append(names, tier.Services...)
	}

	sort.Strings(names)

	return names, nil
}

// serviceDirAbs returns the absolute service directory path
func serviceDirAbs(svc *config.Service) string {
	if filepath.IsAbs(svc.Dir) {
		return svc.Dir
	}

	cwd, err := os.Getwd()
	if err != nil {
		return svc.Dir
	}

	return filepath.Join(cwd, svc.Dir)
}

// skippedServiceResults returns idle placeholders for the per-service checks with the given reason
func skippedServiceResults(reason string) []Result {
	ids := []string{"services.directories", "services.dotenv", "services.readiness"}
	results := make([]Result, 0, len(ids))

	for _, id := range ids {
		results = append(results, Result{
			ID:       id,
			Category: "services",
			Status:   StatusIdle,
			Summary:  "skipped (" + reason + ")",
		})
	}

	return results
}
