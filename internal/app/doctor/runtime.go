package doctor

import (
	"context"
	"fmt"
	"path/filepath"

	"fuku/internal/model"
)

// Runtime observes the sockets and ports other processes hold
type Runtime interface {
	Socket(fingerprint string) model.Socket
	Sockets() model.SocketScan
	ProbePort(ctx context.Context, readiness model.Readiness) model.Port
}

// runtimeSection collects runtime-state checks (sockets, instances, ports)
func (r *Runner) runtimeSection(ctx context.Context, st *state) model.Section {
	return model.Section{
		Title: "Runtime",
		Results: []model.Result{
			timed(func() model.Result { return r.checkInstance(st) }),
			timed(r.checkStaleSockets),
			timed(func() model.Result { return r.checkPorts(ctx, st) }),
		},
	}
}

// checkInstance reports whether another fuku instance is running for the current project
func (r *Runner) checkInstance(st *state) model.Result {
	socket := r.runtime.Socket(st.Fingerprint)

	if !socket.Present {
		return model.Result{
			ID:       model.CheckRuntimeInstance,
			Category: model.CategoryRuntime,
			Severity: model.SeverityIdle,
			Summary:  "no other fuku running for this project",
			Details:  []model.Detail{{Key: "socket", Value: socket.Path + " (absent)"}},
		}
	}

	if !socket.Reachable {
		return model.Result{
			ID:          model.CheckRuntimeInstance,
			Category:    model.CategoryRuntime,
			Severity:    model.SeverityWarn,
			Summary:     "socket present but unreachable",
			Details:     []model.Detail{{Key: "socket", Value: socket.Path}, {Key: "error", Value: socket.Error.Error()}},
			Remediation: "remove the stale socket: rm " + socket.Path,
		}
	}

	return model.Result{
		ID:       model.CheckRuntimeInstance,
		Category: model.CategoryRuntime,
		Severity: model.SeverityNote,
		Summary:  "another fuku is running for this project",
		Details:  []model.Detail{{Key: "socket", Value: socket.Path}},
	}
}

// checkStaleSockets reports stale socket files from previous fuku runs
func (r *Runner) checkStaleSockets() model.Result {
	scan := r.runtime.Sockets()

	var stale []string

	for _, socket := range scan.Sockets {
		if socket.Reachable {
			continue
		}

		stale = append(stale, filepath.Base(socket.Path))
	}

	if len(stale) == 0 {
		return model.Result{
			ID:       model.CheckRuntimeSockets,
			Category: model.CategoryRuntime,
			Severity: model.SeverityOK,
			Summary:  "no stale sockets",
			Details:  []model.Detail{{Key: "scanned", Value: fmt.Sprintf("%s (%d files)", scan.Pattern, scan.Files)}},
		}
	}

	details := make([]model.Detail, 0, len(stale))
	for _, name := range stale {
		details = append(details, model.Detail{Key: name, Value: "stale"})
	}

	return model.Result{
		ID:          model.CheckRuntimeSockets,
		Category:    model.CategoryRuntime,
		Severity:    model.SeverityWarn,
		Summary:     fmt.Sprintf("%d stale socket file(s)", len(stale)),
		Details:     details,
		Remediation: "remove the stale sockets from " + scan.Dir,
	}
}

// checkPorts probes readiness ports for already-bound listeners
func (r *Runner) checkPorts(ctx context.Context, st *state) model.Result {
	if !st.loaded() {
		return skipped(model.CheckRuntimePorts, model.CategoryRuntime, "config did not load")
	}

	if st.profileErr != nil {
		return skipped(model.CheckRuntimePorts, model.CategoryRuntime, "profile did not resolve")
	}

	var (
		probed int
		busy   []model.Detail
	)

	for _, name := range st.services {
		svc, _ := st.Project.Service(name)
		if svc.Readiness == nil {
			continue
		}

		port := r.runtime.ProbePort(ctx, *svc.Readiness)
		if port.Address == "" {
			continue
		}

		probed++

		if port.InUse {
			busy = append(busy, model.Detail{Key: name, Value: port.Address + " already LISTENING"})
		}
	}

	if probed == 0 {
		return model.Result{
			ID:       model.CheckRuntimePorts,
			Category: model.CategoryRuntime,
			Severity: model.SeverityIdle,
			Summary:  "no probed readiness ports",
		}
	}

	if len(busy) > 0 {
		return model.Result{
			ID:          model.CheckRuntimePorts,
			Category:    model.CategoryRuntime,
			Severity:    model.SeverityWarn,
			Summary:     fmt.Sprintf("%d readiness port(s) already bound", len(busy)),
			Details:     busy,
			Remediation: "stop the conflicting process or change the readiness port",
		}
	}

	return model.Result{
		ID:       model.CheckRuntimePorts,
		Category: model.CategoryRuntime,
		Severity: model.SeverityOK,
		Summary:  fmt.Sprintf("%d readiness port(s) available", probed),
	}
}
