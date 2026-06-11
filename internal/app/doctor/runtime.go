package doctor

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"fuku/internal/app/relay"
	"fuku/internal/config"
)

// runtimeSection collects runtime-state checks (sockets, instances, ports)
func runtimeSection(ctx context.Context, env *Env) Section {
	return Section{
		Title: "Runtime",
		Results: []Result{
			timed(func() Result { return checkInstance(env) }),
			timed(checkStaleSockets),
			timed(func() Result { return checkPorts(ctx, env) }),
		},
	}
}

// checkInstance reports whether another fuku instance is running with the same profile
func checkInstance(env *Env) Result {
	socketPath := relay.SocketPathForProfile(config.SocketDir, env.Profile)

	info, err := os.Lstat(socketPath)
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return Result{
			ID:       "runtime.instance",
			Category: "runtime",
			Status:   StatusIdle,
			Summary:  fmt.Sprintf("no other fuku running for profile '%s'", env.Profile),
			Details:  []Detail{{Key: "socket", Value: socketPath + " (absent)"}},
		}
	}

	conn, dialErr := net.DialTimeout("unix", socketPath, config.SocketDialTimeout)
	if dialErr != nil {
		return Result{
			ID:          "runtime.instance",
			Category:    "runtime",
			Status:      StatusWarn,
			Summary:     "socket present but unreachable",
			Details:     []Detail{{Key: "socket", Value: socketPath}, {Key: "error", Value: dialErr.Error()}},
			Remediation: "remove the stale socket: rm " + socketPath,
		}
	}

	conn.Close()

	return Result{
		ID:       "runtime.instance",
		Category: "runtime",
		Status:   StatusNote,
		Summary:  fmt.Sprintf("another fuku is running for profile '%s'", env.Profile),
		Details:  []Detail{{Key: "socket", Value: socketPath}},
	}
}

// checkStaleSockets reports stale socket files from previous fuku runs
func checkStaleSockets() Result {
	pattern := relay.SocketPathForProfile(config.SocketDir, "*")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return Result{
			ID:       "runtime.sockets",
			Category: "runtime",
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
			Category: "runtime",
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
		Category:    "runtime",
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
			Category: "runtime",
			Status:   StatusIdle,
			Summary:  "skipped (config did not load)",
		}
	}

	if env.ProfileErr != nil {
		return Result{
			ID:       "runtime.ports",
			Category: "runtime",
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
			Category: "runtime",
			Status:   StatusIdle,
			Summary:  "no probed readiness ports",
		}
	}

	if len(busy) > 0 {
		return Result{
			ID:          "runtime.ports",
			Category:    "runtime",
			Status:      StatusWarn,
			Summary:     fmt.Sprintf("%d readiness port(s) already bound", len(busy)),
			Details:     busy,
			Remediation: "stop the conflicting process or change the readiness port",
		}
	}

	return Result{
		ID:       "runtime.ports",
		Category: "runtime",
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
