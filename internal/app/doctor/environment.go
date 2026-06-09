package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// environmentSection collects environment and toolchain checks
func environmentSection(_ context.Context, _ *Env) Section {
	return Section{
		Title: "Environment",
		Results: []Result{
			timed(checkSystem),
			timed(checkRuntime),
			timed(checkInstall),
		},
	}
}

// checkSystem reports basic OS and locale information (always OK)
func checkSystem() Result {
	return Result{
		ID:       "system",
		Category: "environment",
		Status:   StatusOK,
		Summary:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		Details: []Detail{
			{Key: "os", Value: runtime.GOOS},
			{Key: "arch", Value: runtime.GOARCH},
			{Key: "shell", Value: os.Getenv("SHELL")},
			{Key: "LANG", Value: envOrDash("LANG")},
		},
	}
}

// checkRuntime reports the active Go runtime version
func checkRuntime() Result {
	return Result{
		ID:       "runtime",
		Category: "environment",
		Status:   StatusOK,
		Summary:  runtime.Version(),
		Details: []Detail{
			{Key: "go version", Value: runtime.Version()},
			{Key: "GOOS", Value: runtime.GOOS},
			{Key: "GOARCH", Value: runtime.GOARCH},
		},
	}
}

// checkInstall reports the resolved fuku executable path
func checkInstall() Result {
	exe, exeErr := os.Executable()
	if exeErr != nil {
		return Result{
			ID:          "install",
			Category:    "environment",
			Status:      StatusWarn,
			Summary:     "could not resolve fuku executable",
			Remediation: "ensure fuku binary is reachable on PATH",
		}
	}

	details := []Detail{
		{Key: "executable", Value: exe},
	}

	if pathExe, err := exec.LookPath("fuku"); err == nil {
		details = append(details, Detail{Key: "PATH fuku", Value: pathExe})
	}

	return Result{
		ID:       "install",
		Category: "environment",
		Status:   StatusOK,
		Summary:  "installation looks consistent",
		Details:  details,
	}
}

// envOrDash returns the env var value or "-" when unset
func envOrDash(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return "-"
}
