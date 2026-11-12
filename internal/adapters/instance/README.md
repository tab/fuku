# adapters/instance

Who this fuku is, and the guard that keeps one fuku per project.

## Identity

`NewInstance` builds `model.Instance{ID, Project, Fingerprint}` once per invocation:

- `ID` is a fresh UUID
- `Project` is the working directory, absolute, with symlinks resolved. `Project(dir)` does the same for any directory
- `Fingerprint` is the first 16 hex characters of the SHA-256 of `Project`. It names the project without showing the path

The MCP server resolves the project of each call with `Project(dir)`, so its fingerprint matches the one of the instance.

The project socket is `/tmp/fuku-<fingerprint>.sock`. `SocketPath` and `SocketDir` are its one definition inside `fuku`.
The e2e suite repeats the formula, because it never imports `internal`. `ProbeSocket` is the one liveness dial.
It fails when no process answers within 100ms.

`UserConfigPath(name)` is the one place for `$UserConfigDir/fuku/<name>`. The update cache and the telemetry ID live there.
Whoever writes first creates the directory with `0700`.

## The guard

`Guard.Check` runs first in the run composition. It takes an exclusive `flock` on `/tmp/fuku-<fingerprint>.lock`. The lock is the only decision:

- the lock is free: the run owns the project. The guard keeps the file open for the life of the process
- another run holds it: `Check` refuses with `ErrInstanceAlreadyRunning`

The kernel releases the lock when the process exits, however it exits. The lock file is never removed.
An unlinked file would let a second run lock a new file under the same name.

The refusal goes to stderr and names where the other run answers:

```text
Error: fuku is already running for this project (socket /tmp/fuku-<fingerprint>.sock)
Run 'fuku logs' to follow it or stop that instance before starting another.
```

When the socket of the other run is not up yet, the refusal names `lock /tmp/fuku-<fingerprint>.lock` instead.
A file that cannot be opened or locked fails the start with `ErrFailedToLockProject`.

## Changing it

- the fingerprint, the socket name and the lock name are a compatibility contract. A running instance and a newer `fuku logs` must agree on them
- the lock decides. Never add a probe that refuses a run the lock admitted
- keep the lock file open on the guard. A dropped `*os.File` is closed by its finalizer, and that releases the lock
