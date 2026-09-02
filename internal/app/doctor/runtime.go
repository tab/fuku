package doctor

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"fuku/internal/app/instance"
	"fuku/internal/app/relay"
	"fuku/internal/config"
)

// runtimeSection collects runtime-state checks (sockets, instances, ports)
func runtimeSection(ctx context.Context, env *Env) Section {
	return Section{
		Title: "Runtime",
		Results: []Result{
			timed(func() Result { return checkInstance(ctx, env) }),
			timed(checkStaleSockets),
			timed(func() Result { return checkAPI(ctx, env) }),
			timed(func() Result { return checkPorts(ctx, env) }),
		},
	}
}

// checkInstance reports whether another fuku instance holds the log socket for this profile
// (the socket directory is shared by every project on the machine, so the banner is read to
// attribute the instance rather than assuming the profile name belongs to this project)
func checkInstance(ctx context.Context, env *Env) Result {
	socketPath := relay.SocketPathForProfile(config.SocketDir, env.Profile)

	info, err := os.Lstat(socketPath)
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return Result{
			ID:       "runtime.instance",
			Category: CategoryRuntime,
			Status:   StatusIdle,
			Summary:  fmt.Sprintf("no other fuku running for profile '%s'", env.Profile),
			Details:  []Detail{{Key: "socket", Value: socketPath + " (absent)"}},
		}
	}

	status, identifyErr := relay.Identify(ctx, socketPath)
	if identifyErr != nil {
		return Result{
			ID:          "runtime.instance",
			Category:    CategoryRuntime,
			Status:      StatusWarn,
			Summary:     "socket present but unreachable",
			Details:     []Detail{{Key: "socket", Value: socketPath}, {Key: "error", Value: identifyErr.Error()}},
			Remediation: "remove the stale socket: rm " + socketPath,
		}
	}

	return instanceResult(env, socketPath, status)
}

// instanceResult describes the instance holding this profile's socket, relative to this project
func instanceResult(env *Env, socketPath string, status relay.StatusMessage) Result {
	details := []Detail{{Key: "socket", Value: socketPath}}

	summary := fmt.Sprintf("another fuku is running for profile '%s'", env.Profile)
	if status.Project != "" && env.Fingerprint != "" && status.Project != env.Fingerprint {
		summary = fmt.Sprintf("profile '%s' socket belongs to another project", env.Profile)

		details = append(details, Detail{Key: "project", Value: "another directory"})
	}

	if status.Project == "" {
		details = append(details, Detail{Key: "project", Value: "not reported by that instance"})
	}

	return Result{
		ID:       "runtime.instance",
		Category: CategoryRuntime,
		Status:   StatusNote,
		Summary:  summary,
		Details:  details,
	}
}

// checkAPI reports which loopback address this project's running instance actually bound
func checkAPI(ctx context.Context, env *Env) Result {
	if env.Config == nil {
		return Result{
			ID:       "runtime.api",
			Category: CategoryRuntime,
			Status:   StatusIdle,
			Summary:  summarySkippedNoConfig,
		}
	}

	listen := env.Config.ServerListen()
	if listen == "" {
		return Result{
			ID:       "runtime.api",
			Category: CategoryRuntime,
			Status:   StatusIdle,
			Summary:  "skipped (server.listen is not configured)",
		}
	}

	return apiResult(env, listen, instance.Scan(ctx, listen))
}

// apiResult describes the instances answering in the port range, relative to this project
func apiResult(env *Env, listen string, found []instance.Instance) Result {
	for _, candidate := range found {
		if env.Fingerprint == "" || candidate.Project != env.Fingerprint {
			continue
		}

		details := []Detail{
			{Key: "bound", Value: candidate.Address},
			{Key: "configured", Value: listen},
			{Key: "instance", Value: candidate.ID},
		}

		return Result{
			ID:       "runtime.api",
			Category: CategoryRuntime,
			Status:   StatusOK,
			Summary:  "this project's instance is answering on " + candidate.Address,
			Details:  details,
		}
	}

	if len(found) == 0 {
		return Result{
			ID:       "runtime.api",
			Category: CategoryRuntime,
			Status:   StatusIdle,
			Summary:  "no instance answering on " + portRange(listen),
		}
	}

	details := make([]Detail, 0, len(found))
	unidentified := 0

	for _, candidate := range found {
		if candidate.Project == "" {
			unidentified++

			details = append(details, Detail{Key: candidate.Address, Value: "did not report its project"})

			continue
		}

		details = append(details, Detail{Key: candidate.Address, Value: "serves another project"})
	}

	remediation := "run the profile from this directory, or give this project its own server.listen range"
	if unidentified == len(found) {
		remediation = "upgrade fuku so the instance reports the project it serves"
	}

	return Result{
		ID:          "runtime.api",
		Category:    CategoryRuntime,
		Status:      StatusNote,
		Summary:     fmt.Sprintf("%d instance(s) on %s do not belong to this project", len(found), portRange(listen)),
		Details:     details,
		Remediation: remediation,
	}
}

// checkStaleSockets reports stale socket files from previous fuku runs
func checkStaleSockets() Result {
	pattern := relay.SocketPathForProfile(config.SocketDir, "*")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return Result{
			ID:       "runtime.sockets",
			Category: CategoryRuntime,
			Status:   StatusWarn,
			Summary:  "failed to glob socket directory",
			Details:  []Detail{{Key: "error", Value: err.Error()}},
		}
	}

	var stale []string

	for _, socketPath := range matches {
		info, err := os.Lstat(socketPath)
		if err != nil || info.Mode()&os.ModeSocket == 0 {
			continue
		}

		conn, err := net.DialTimeout("unix", socketPath, config.SocketDialTimeout)
		if err == nil {
			conn.Close()
			continue
		}

		stale = append(stale, filepath.Base(socketPath))
	}

	if len(stale) == 0 {
		return Result{
			ID:       "runtime.sockets",
			Category: CategoryRuntime,
			Status:   StatusOK,
			Summary:  "no stale sockets",
			Details:  []Detail{{Key: "scanned", Value: fmt.Sprintf("%s (%d files)", pattern, len(matches))}},
		}
	}

	details := make([]Detail, 0, len(stale))
	for _, name := range stale {
		details = append(details, Detail{Key: name, Value: "stale"})
	}

	return Result{
		ID:          "runtime.sockets",
		Category:    CategoryRuntime,
		Status:      StatusWarn,
		Summary:     fmt.Sprintf("%d stale socket file(s)", len(stale)),
		Details:     details,
		Remediation: "remove the stale sockets from " + config.SocketDir,
	}
}

// checkPorts probes readiness ports for already-bound listeners
func checkPorts(ctx context.Context, env *Env) Result {
	if env.Config == nil {
		return Result{
			ID:       "runtime.ports",
			Category: CategoryRuntime,
			Status:   StatusIdle,
			Summary:  summarySkippedNoConfig,
		}
	}

	if env.ProfileErr != nil {
		return Result{
			ID:       "runtime.ports",
			Category: CategoryRuntime,
			Status:   StatusIdle,
			Summary:  "skipped (profile did not resolve)",
		}
	}

	names := env.ProfileServices

	var (
		probed int
		busy   []Detail
	)

	dialer := net.Dialer{Timeout: config.PreFlightTimeout}

	for _, name := range names {
		svc := env.Config.Services[name]
		if svc.Readiness == nil {
			continue
		}

		address := extractAddress(svc.Readiness)
		if address == "" {
			continue
		}

		probed++

		conn, dialErr := dialer.DialContext(ctx, "tcp", address)
		if dialErr != nil {
			continue
		}

		conn.Close()

		busy = append(busy, Detail{Key: name, Value: address + " already LISTENING"})
	}

	if probed == 0 {
		return Result{
			ID:       "runtime.ports",
			Category: CategoryRuntime,
			Status:   StatusIdle,
			Summary:  "no probed readiness ports",
		}
	}

	if len(busy) > 0 {
		return Result{
			ID:          "runtime.ports",
			Category:    CategoryRuntime,
			Status:      StatusWarn,
			Summary:     fmt.Sprintf("%d readiness port(s) already bound", len(busy)),
			Details:     busy,
			Remediation: "stop the conflicting process or change the readiness port",
		}
	}

	return Result{
		ID:       "runtime.ports",
		Category: CategoryRuntime,
		Status:   StatusOK,
		Summary:  fmt.Sprintf("%d readiness port(s) available", probed),
	}
}

// portOrDefault returns the explicit URL port, falling back to scheme defaults (80/443)
func portOrDefault(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}

	if u.Scheme == "https" {
		return "443"
	}

	return "80"
}

// extractAddress derives host:port from a readiness probe configuration
func extractAddress(r *config.Readiness) string {
	switch r.Type {
	case config.TypeTCP:
		return r.Address
	case config.TypeHTTP:
		u, err := url.Parse(r.URL)
		if err != nil || u.Host == "" {
			return ""
		}

		if u.Scheme != "http" && u.Scheme != "https" {
			return ""
		}

		return net.JoinHostPort(u.Hostname(), portOrDefault(u))
	default:
		return ""
	}
}
