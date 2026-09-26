package diagnostics

import (
	"context"
	"os"
	"path/filepath"

	"fuku/internal/adapters/instance"
	"fuku/internal/adapters/readiness"
	"fuku/internal/model"
)

// Runtime observes the sockets and ports other processes hold
type Runtime struct{}

// NewRuntime creates an observer of the runtime state
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Socket reports whether the socket file of the fingerprinted project exists and whether an instance answers on it
func (r *Runtime) Socket(fingerprint string) model.Socket {
	return socketAt(instance.SocketPath(instance.SocketDir, fingerprint))
}

// socketAt reports whether the socket file at path exists and whether an instance answers on it
func socketAt(path string) model.Socket {
	socket := model.Socket{Path: path}

	if !isSocket(socket.Path) {
		return socket
	}

	socket.Present = true
	socket.Error = instance.ProbeSocket(socket.Path)
	socket.Reachable = socket.Error == nil

	return socket
}

// Sockets lists every fuku socket file in the socket directory with whether an instance answers on it
func (r *Runtime) Sockets() model.SocketScan {
	return scanSockets(instance.SocketDir)
}

// scanSockets lists every fuku socket file in dir with whether an instance answers on it
func scanSockets(dir string) model.SocketScan {
	scan := model.SocketScan{Dir: dir, Pattern: instance.SocketPath(dir, "*")}

	matches, _ := filepath.Glob(scan.Pattern)

	scan.Files = len(matches)

	for _, path := range matches {
		if !isSocket(path) {
			continue
		}

		socket := model.Socket{Path: path, Present: true, Error: instance.ProbeSocket(path)}
		socket.Reachable = socket.Error == nil

		scan.Sockets = append(scan.Sockets, socket)
	}

	return scan
}

// ProbePort reports the address of the readiness probe and whether it is in use through the one readiness port probe
func (r *Runtime) ProbePort(_ context.Context, probe model.Readiness) model.Port {
	return readiness.ProbePort(probe)
}

// isSocket reports whether path is a unix socket file
func isSocket(path string) bool {
	info, err := os.Lstat(path)

	return err == nil && info.Mode()&os.ModeSocket != 0
}
