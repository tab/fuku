package doctor

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"regexp"

	"fuku/internal/model"
)

// Filesystem observes the working directory and the paths a project references
type Filesystem interface {
	Getwd() (string, error)
	DirExists(path string) bool
	FileExists(path string) bool
}

// servicesSection collects per-service filesystem and field checks
func (r *Runner) servicesSection(st *state) model.Section {
	section := model.Section{
		Title: "Services",
	}

	if !st.loaded() {
		section.Note = "skipped (config did not load)"
		section.Results = skippedServiceResults("config did not load")

		return section
	}

	if st.profileErr != nil {
		section.Note = "skipped (profile did not resolve)"
		section.Results = skippedServiceResults("profile did not resolve")

		return section
	}

	names := st.services

	section.Note = fmt.Sprintf("active profile: %s · %d services", st.Profile, len(names))
	section.Results = []model.Result{
		timed(func() model.Result { return r.checkServiceDirectories(st, names) }),
		timed(func() model.Result { return r.checkServiceDotenv(st, names) }),
		timed(func() model.Result { return checkServiceReadiness(st, names) }),
	}

	return section
}

// checkServiceDirectories verifies that each service.Dir exists on disk
func (r *Runner) checkServiceDirectories(st *state, names []string) model.Result {
	var missing []string

	details := make([]model.Detail, 0, len(names))

	for _, name := range names {
		svc, _ := st.Project.Service(name)
		dir := r.serviceDirAbs(svc)

		if r.filesystem.DirExists(dir) {
			details = append(details, model.Detail{Key: name, Value: dir})

			continue
		}

		missing = append(missing, name)
		details = append(details, model.Detail{Key: name, Value: dir + " (MISSING)"})
	}

	if len(missing) == 0 {
		return model.Result{
			ID:       model.CheckServicesDirectories,
			Category: model.CategoryServices,
			Severity: model.SeverityOK,
			Summary:  fmt.Sprintf("%d of %d directories present", len(names), len(names)),
			Details:  details,
		}
	}

	return model.Result{
		ID:          model.CheckServicesDirectories,
		Category:    model.CategoryServices,
		Severity:    model.SeverityWarn,
		Summary:     fmt.Sprintf("%d of %d directories missing", len(missing), len(names)),
		Details:     details,
		Remediation: "create the missing directories or fix `dir:` paths in fuku.yaml",
	}
}

// checkServiceDotenv verifies that each referenced .env file exists and is readable
func (r *Runner) checkServiceDotenv(st *state, names []string) model.Result {
	var missing []string

	total := 0
	details := []model.Detail{}

	for _, name := range names {
		svc, _ := st.Project.Service(name)
		if svc.Environment.Defaulted || len(svc.Environment.Files) == 0 {
			continue
		}

		dir := r.serviceDirAbs(svc)

		for _, file := range svc.Environment.Files {
			total++

			path := filepath.Join(dir, file)
			if r.filesystem.FileExists(path) {
				continue
			}

			label := fmt.Sprintf("%s/%s", name, file)
			missing = append(missing, label)
			details = append(details, model.Detail{Key: label, Value: "MISSING"})
		}
	}

	if total == 0 {
		return model.Result{
			ID:       model.CheckServicesDotenv,
			Category: model.CategoryServices,
			Severity: model.SeverityIdle,
			Summary:  "no .env files referenced",
		}
	}

	if len(missing) == 0 {
		return model.Result{
			ID:       model.CheckServicesDotenv,
			Category: model.CategoryServices,
			Severity: model.SeverityOK,
			Summary:  fmt.Sprintf("%d files referenced, all readable", total),
		}
	}

	return model.Result{
		ID:          model.CheckServicesDotenv,
		Category:    model.CategoryServices,
		Severity:    model.SeverityWarn,
		Summary:     fmt.Sprintf("%d of %d referenced .env files missing", len(missing), total),
		Details:     details,
		Remediation: "create the missing .env files or update `env.files` in fuku.yaml",
	}
}

// checkServiceReadiness verifies probe fields parse as URL, regex, or host:port
func checkServiceReadiness(st *state, names []string) model.Result {
	var (
		http, tcp, log int
		issues         []model.Detail
	)

	for _, name := range names {
		svc, _ := st.Project.Service(name)
		if svc.Readiness == nil {
			continue
		}

		probe := svc.Readiness

		switch probe.Type {
		case model.ReadinessHTTP:
			http++

			if err := validateHTTPURL(probe.URL); err != nil {
				issues = append(issues, model.Detail{Key: name + " url", Value: err.Error()})
			}
		case model.ReadinessTCP:
			tcp++

			if _, _, err := net.SplitHostPort(probe.Address); err != nil {
				issues = append(issues, model.Detail{Key: name + " address", Value: err.Error()})
			}
		case model.ReadinessLog:
			log++

			if _, err := regexp.Compile(probe.Pattern); err != nil {
				issues = append(issues, model.Detail{Key: name + " pattern", Value: err.Error()})
			}
		}
	}

	total := http + tcp + log
	if total == 0 {
		return model.Result{
			ID:       model.CheckServicesReadiness,
			Category: model.CategoryServices,
			Severity: model.SeverityIdle,
			Summary:  "no readiness probes defined",
		}
	}

	if len(issues) > 0 {
		return model.Result{
			ID:          model.CheckServicesReadiness,
			Category:    model.CategoryServices,
			Severity:    model.SeverityFail,
			Summary:     fmt.Sprintf("%d probe field(s) failed to parse", len(issues)),
			Details:     issues,
			Remediation: "fix the malformed readiness fields in fuku.yaml",
		}
	}

	return model.Result{
		ID:       model.CheckServicesReadiness,
		Category: model.CategoryServices,
		Severity: model.SeverityOK,
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

// serviceDirAbs returns the absolute service directory path
func (r *Runner) serviceDirAbs(svc model.Service) string {
	if filepath.IsAbs(svc.Directory) {
		return svc.Directory
	}

	cwd, err := r.filesystem.Getwd()
	if err != nil {
		return svc.Directory
	}

	return filepath.Join(cwd, svc.Directory)
}

// skippedServiceResults returns idle placeholders for the per-service checks with the given reason
func skippedServiceResults(reason string) []model.Result {
	ids := []model.CheckID{model.CheckServicesDirectories, model.CheckServicesDotenv, model.CheckServicesReadiness}
	results := make([]model.Result, 0, len(ids))

	for _, id := range ids {
		results = append(results, skipped(id, model.CategoryServices, reason))
	}

	return results
}
