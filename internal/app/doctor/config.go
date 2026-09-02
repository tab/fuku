package doctor

import (
	"context"
	"fmt"
	"strconv"

	"fuku/internal/app/errors"
	"fuku/internal/app/instance"
	"fuku/internal/config"
)

// configSection collects config-file and validation checks
func configSection(_ context.Context, env *Env) Section {
	return Section{
		Title: "Configuration",
		Results: []Result{
			timed(func() Result { return checkConfigFile(env) }),
			timed(func() Result { return checkConfigOverride(env) }),
			timed(func() Result { return checkConfigValidate(env) }),
			timed(func() Result { return checkConfigSettings(env) }),
			timed(func() Result { return checkConfigAPI(env) }),
		},
	}
}

// checkConfigFile reports whether the base config file was found and loaded
func checkConfigFile(env *Env) Result {
	if env.ConfigPath == "" {
		return Result{
			ID:          "config.file",
			Category:    CategoryConfiguration,
			Status:      StatusFail,
			Summary:     "no fuku.yaml found in current directory",
			Remediation: "run `fuku init` to generate a template",
		}
	}

	// A schema-validation failure means the file was found, read, and parsed; that
	// is reported by config.validate, so this check only flags read/parse failures.
	if env.LoadErr != nil && !errors.Is(env.LoadErr, errors.ErrInvalidConfig) {
		return Result{
			ID:       "config.file",
			Category: CategoryConfiguration,
			Status:   StatusFail,
			Summary:  "failed to load " + env.ConfigPath,
			Details: []Detail{
				{Key: "path", Value: env.ConfigPath},
				{Key: "error", Value: env.LoadErr.Error()},
			},
			Remediation: "check YAML syntax and required fields",
		}
	}

	return Result{
		ID:       "config.file",
		Category: CategoryConfiguration,
		Status:   StatusOK,
		Summary:  "found and parsed",
		Details: []Detail{
			{Key: "path", Value: env.ConfigPath},
		},
	}
}

// checkConfigOverride reports whether an override file is present and whether it was merged
func checkConfigOverride(env *Env) Result {
	if env.OverridePath == "" {
		return Result{
			ID:       "config.override",
			Category: CategoryConfiguration,
			Status:   StatusIdle,
			Summary:  "no override file present",
		}
	}

	if env.ExplicitConfig {
		return Result{
			ID:       "config.override",
			Category: CategoryConfiguration,
			Status:   StatusNote,
			Summary:  "override file present but skipped (--config bypasses overrides)",
			Details: []Detail{
				{Key: "path", Value: env.OverridePath},
			},
		}
	}

	if env.LoadErr != nil {
		return Result{
			ID:       "config.override",
			Category: CategoryConfiguration,
			Status:   StatusIdle,
			Summary:  "override merge status unknown (config did not load)",
			Details: []Detail{
				{Key: "path", Value: env.OverridePath},
			},
		}
	}

	return Result{
		ID:       "config.override",
		Category: CategoryConfiguration,
		Status:   StatusOK,
		Summary:  "override applied",
		Details: []Detail{
			{Key: "path", Value: env.OverridePath},
		},
	}
}

// checkConfigValidate reports the result of schema validation
func checkConfigValidate(env *Env) Result {
	// Load() validates internally and returns a nil Config with ErrInvalidConfig on
	// failure, so a validation error arrives via LoadErr rather than env.Config.
	if errors.Is(env.LoadErr, errors.ErrInvalidConfig) {
		return invalidConfigResult(env.LoadErr)
	}

	if env.Config == nil {
		return Result{
			ID:       "config.validate",
			Category: CategoryConfiguration,
			Status:   StatusIdle,
			Summary:  summarySkippedNoConfig,
		}
	}

	if err := env.Config.Validate(); err != nil {
		return invalidConfigResult(err)
	}

	return Result{
		ID:       "config.validate",
		Category: CategoryConfiguration,
		Status:   StatusOK,
		Summary:  "schema ok",
	}
}

// invalidConfigResult builds the config.validate failure result for a schema error
func invalidConfigResult(err error) Result {
	return Result{
		ID:       "config.validate",
		Category: CategoryConfiguration,
		Status:   StatusFail,
		Summary:  "schema validation failed",
		Details: []Detail{
			{Key: "error", Value: err.Error()},
		},
		Remediation: "fix the offending field in fuku.yaml",
	}
}

// checkConfigSettings reports concurrency, retry, and log buffer settings
func checkConfigSettings(env *Env) Result {
	if env.Config == nil {
		return Result{
			ID:       "config.settings",
			Category: CategoryConfiguration,
			Status:   StatusIdle,
			Summary:  summarySkippedNoConfig,
		}
	}

	cfg := env.Config

	return Result{
		ID:       "config.settings",
		Category: CategoryConfiguration,
		Status:   StatusOK,
		Summary: fmt.Sprintf("workers=%d retry=%d backoff=%s",
			cfg.Concurrency.Workers, cfg.Retry.Attempts, cfg.Retry.Backoff),
		Details: []Detail{
			{Key: "concurrency workers", Value: strconv.Itoa(cfg.Concurrency.Workers)},
			{Key: "retry attempts", Value: strconv.Itoa(cfg.Retry.Attempts)},
			{Key: "retry backoff", Value: cfg.Retry.Backoff.String()},
			{Key: "logs buffer", Value: strconv.Itoa(cfg.Logs.Buffer)},
			{Key: "logs history", Value: strconv.Itoa(cfg.Logs.History)},
			{Key: "logging level", Value: cfg.Logging.Level},
			{Key: "logging format", Value: cfg.Logging.Format},
		},
	}
}

// checkConfigAPI reports whether the built-in HTTP API is configured
// (schema validation already rejects a listen address without a token or off the loopback interface,
// so a config that loaded is either silent about the server or fully valid)
func checkConfigAPI(env *Env) Result {
	if env.Config == nil {
		return Result{
			ID:       "config.api",
			Category: CategoryConfiguration,
			Status:   StatusIdle,
			Summary:  summarySkippedNoConfig,
		}
	}

	listen := env.Config.ServerListen()
	if listen == "" {
		return Result{
			ID:       "config.api",
			Category: CategoryConfiguration,
			Status:   StatusIdle,
			Summary:  "not configured (no REST API, and `fuku run` cannot detect a second instance)",
			Remediation: "set server.listen and server.auth.token in fuku.override.yaml " +
				"so the token stays out of the committed config",
		}
	}

	details := []Detail{
		{Key: "listen", Value: listen},
		{Key: "auth token", Value: "set"},
	}

	if ports := portRange(listen); ports != "" {
		details = append(details, Detail{Key: "port fallback", Value: ports})
	}

	return Result{
		ID:       "config.api",
		Category: CategoryConfiguration,
		Status:   StatusOK,
		Summary:  "enabled on " + listen,
		Details:  details,
	}
}

// portRange describes the ports fuku may bind when the configured one is taken
func portRange(listen string) string {
	_, port, err := instance.SplitListen(listen)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%d-%d", port, port+config.APIPortRetries-1)
}
