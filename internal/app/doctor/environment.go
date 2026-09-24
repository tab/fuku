package doctor

import (
	"fmt"
	"runtime"

	"fuku/internal/model"
)

// Environment observes the process environment and the fuku installation
type Environment interface {
	Getenv(key string) string
	Executable() (string, error)
	PathExecutable() (string, error)
}

// environmentSection collects environment and toolchain checks
func (r *Runner) environmentSection() model.Section {
	return model.Section{
		Title: "Environment",
		Results: []model.Result{
			timed(r.checkSystem),
			timed(checkRuntime),
			timed(r.checkInstall),
		},
	}
}

// checkSystem reports basic OS and locale information (always OK)
func (r *Runner) checkSystem() model.Result {
	return model.Result{
		ID:       model.CheckSystem,
		Category: model.CategoryEnvironment,
		Severity: model.SeverityOK,
		Summary:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		Details: []model.Detail{
			{Key: "os", Value: runtime.GOOS},
			{Key: "arch", Value: runtime.GOARCH},
			{Key: "shell", Value: r.environment.Getenv("SHELL")},
			{Key: "LANG", Value: r.envOrDash("LANG")},
		},
	}
}

// checkRuntime reports the active Go runtime version
func checkRuntime() model.Result {
	return model.Result{
		ID:       model.CheckRuntime,
		Category: model.CategoryEnvironment,
		Severity: model.SeverityOK,
		Summary:  runtime.Version(),
		Details: []model.Detail{
			{Key: "go version", Value: runtime.Version()},
			{Key: "GOOS", Value: runtime.GOOS},
			{Key: "GOARCH", Value: runtime.GOARCH},
		},
	}
}

// checkInstall reports the resolved fuku executable path
func (r *Runner) checkInstall() model.Result {
	exe, exeErr := r.environment.Executable()
	if exeErr != nil {
		return model.Result{
			ID:          model.CheckInstall,
			Category:    model.CategoryEnvironment,
			Severity:    model.SeverityWarn,
			Summary:     "could not resolve fuku executable",
			Remediation: "ensure fuku binary is reachable on PATH",
		}
	}

	details := []model.Detail{
		{Key: "executable", Value: exe},
	}

	if pathExe, err := r.environment.PathExecutable(); err == nil {
		details = append(details, model.Detail{Key: "PATH fuku", Value: pathExe})
	}

	return model.Result{
		ID:       model.CheckInstall,
		Category: model.CategoryEnvironment,
		Severity: model.SeverityOK,
		Summary:  "installation looks consistent",
		Details:  details,
	}
}

// envOrDash returns the env var value or "-" when unset
func (r *Runner) envOrDash(key string) string {
	if v := r.environment.Getenv(key); v != "" {
		return v
	}

	return "-"
}
