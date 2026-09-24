# adapters/diagnostics

The observers `doctor` reads the machine through. Each one reports a fact. Findings, severity and the report stay in `app/doctor`.
`bootstrap/modules/doctor.go` binds `Environment`, `Filesystem` and `Runtime` to the `doctor` interfaces of the same names.

## How it works

- `Environment` reads a variable, the running binary and the `fuku` a `PATH` lookup finds
- `Filesystem` reads the working directory and whether a path is a file or a directory
- a socket is present when `Lstat` shows a unix socket. It is reachable when a dial answers within `instance.SocketDialTimeout`
- `Sockets` globs the socket pattern in `instance.SocketDir` and tests each match the same way
- `ProbePort` is `readiness.ProbePort`. The 100ms probe timeout and the default ports (`http` 80, `https` 443) live there

## Changing it

- an observer returns a value, never a verdict
- a port probe changes in `readiness`. Then `doctor` and the pre-launch port check agree
