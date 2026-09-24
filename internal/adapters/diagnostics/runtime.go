package diagnostics

import (
	"context"
	"net"
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
	socket := model.Socket{Path: instance.SocketPath(instance.SocketDir, fingerprint)}

	if !isSocket(socket.Path) {
		return socket
	}

	socket.Present = true
	socket.Reachable, socket.Error = dial(socket.Path)

	return socket
}

// Sockets lists every fuku socket file in the socket directory with whether an instance answers on it
func (r *Runtime) Sockets() model.SocketScan {
	scan := model.SocketScan{Dir: instance.SocketDir, Pattern: instance.SocketPath(instance.SocketDir, "*")}

	matches, _ := filepath.Glob(scan.Pattern)

	scan.Files = len(matches)

	for _, path := range matches {
		if !isSocket(path) {
			continue
		}

		socket := model.Socket{Path: path, Present: true}
		socket.Reachable, socket.Error = dial(path)

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

// dial reports whether a process answers on the unix socket at path
func dial(path string) (bool, error) {
	conn, err := net.DialTimeout("unix", path, instance.SocketDialTimeout)
	if err != nil {
		return false, err
	}

	conn.Close()

	return true, nil
}
