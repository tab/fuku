package doctor

import (
	"errors"
	"fmt"
	"strconv"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// configSection collects config-file and validation checks
func configSection(st *state) model.Section {
	return model.Section{
		Title: "Configuration",
		Results: []model.Result{
			timed(func() model.Result { return checkConfigFile(st) }),
			timed(func() model.Result { return checkConfigOverride(st) }),
			timed(func() model.Result { return checkConfigValidate(st) }),
			timed(func() model.Result { return checkConfigSettings(st) }),
		},
	}
}

// checkConfigFile reports whether the base config file was found and loaded
func checkConfigFile(st *state) model.Result {
	if st.Path == "" {
		return model.Result{
			ID:          model.CheckConfigFile,
			Category:    model.CategoryConfiguration,
			Severity:    model.SeverityFail,
			Summary:     "no fuku.yaml found in current directory",
			Remediation: "run `fuku init` to generate a template",
		}
	}

	if st.Error != nil && !errors.Is(st.Error, contracts.ErrInvalidConfig) {
		return model.Result{
			ID:       model.CheckConfigFile,
			Category: model.CategoryConfiguration,
			Severity: model.SeverityFail,
			Summary:  "failed to load " + st.Path,
			Details: []model.Detail{
				{Key: "path", Value: st.Path},
				{Key: "error", Value: st.Error.Error()},
			},
			Remediation: "check YAML syntax and required fields",
		}
	}

	return model.Result{
		ID:       model.CheckConfigFile,
		Category: model.CategoryConfiguration,
		Severity: model.SeverityOK,
		Summary:  "found and parsed",
		Details: []model.Detail{
			{Key: "path", Value: st.Path},
		},
	}
}

// checkConfigOverride reports whether an override file is present and whether it was merged
func checkConfigOverride(st *state) model.Result {
	if st.OverridePath == "" {
		return model.Result{
			ID:       model.CheckConfigOverride,
			Category: model.CategoryConfiguration,
			Severity: model.SeverityIdle,
			Summary:  "no override file present",
		}
	}

	if st.ExplicitConfig {
		return model.Result{
			ID:       model.CheckConfigOverride,
			Category: model.CategoryConfiguration,
			Severity: model.SeverityNote,
			Summary:  "override file present but skipped (--config bypasses overrides)",
			Details: []model.Detail{
				{Key: "path", Value: st.OverridePath},
			},
		}
	}

	if st.Error != nil {
		return model.Result{
			ID:       model.CheckConfigOverride,
			Category: model.CategoryConfiguration,
			Severity: model.SeverityIdle,
			Summary:  "override merge status unknown (config did not load)",
			Details: []model.Detail{
				{Key: "path", Value: st.OverridePath},
			},
		}
	}

	return model.Result{
		ID:       model.CheckConfigOverride,
		Category: model.CategoryConfiguration,
		Severity: model.SeverityOK,
		Summary:  "override applied",
		Details: []model.Detail{
			{Key: "path", Value: st.OverridePath},
		},
	}
}

// checkConfigValidate reports the result of schema validation
func checkConfigValidate(st *state) model.Result {
	if errors.Is(st.Error, contracts.ErrInvalidConfig) {
		return invalidConfigResult(st.Error)
	}

	if !st.loaded() {
		return skipped(model.CheckConfigValidate, model.CategoryConfiguration, "config did not load")
	}

	return model.Result{
		ID:       model.CheckConfigValidate,
		Category: model.CategoryConfiguration,
		Severity: model.SeverityOK,
		Summary:  "schema ok",
	}
}

// invalidConfigResult builds the config.validate failure result for a schema error
func invalidConfigResult(err error) model.Result {
	return model.Result{
		ID:       model.CheckConfigValidate,
		Category: model.CategoryConfiguration,
		Severity: model.SeverityFail,
		Summary:  "schema validation failed",
		Details: []model.Detail{
			{Key: "error", Value: err.Error()},
		},
		Remediation: "fix the offending field in fuku.yaml",
	}
}

// checkConfigSettings reports concurrency, retry, and log buffer settings
func checkConfigSettings(st *state) model.Result {
	if !st.loaded() {
		return skipped(model.CheckConfigSettings, model.CategoryConfiguration, "config did not load")
	}

	project := st.Project

	return model.Result{
		ID:       model.CheckConfigSettings,
		Category: model.CategoryConfiguration,
		Severity: model.SeverityOK,
		Summary: fmt.Sprintf("workers=%d retry=%d backoff=%s",
			project.Concurrency.Workers, project.Retry.Attempts, project.Retry.Backoff),
		Details: []model.Detail{
			{Key: "concurrency workers", Value: strconv.Itoa(project.Concurrency.Workers)},
			{Key: "retry attempts", Value: strconv.Itoa(project.Retry.Attempts)},
			{Key: "retry backoff", Value: project.Retry.Backoff.String()},
			{Key: "logs buffer", Value: strconv.Itoa(project.Logs.Buffer)},
			{Key: "logs history", Value: strconv.Itoa(project.Logs.History)},
			{Key: "logging level", Value: project.Logging.Level},
			{Key: "logging format", Value: project.Logging.Format},
		},
	}
}
