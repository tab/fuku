# adapters/instance

Who this fuku is, and the guard that keeps one fuku per project.

## Identity

`NewInstance` builds `model.Instance{ID, Project, Fingerprint}` once per invocation:

- `ID` is a fresh UUID
- `Project` is the working directory with symlinks resolved
- `Fingerprint` is the first 16 hex characters of the SHA-256 of `Project`. It names the project without showing the path

The project socket is `/tmp/fuku-<fingerprint>.sock`. `SocketPath` and `SocketDir` are the one definition.
`ProbeSocket` is the one liveness dial. It fails when no process answers within 100ms.
`logsocket` and `diagnostics` use them.

`UserConfigPath(name)` is the one place for `$UserConfigDir/fuku/<name>`. The update cache and the telemetry ID live there.
Whoever writes first creates the directory with `0700`.

## The guard

`Guard.Check` runs first in the run composition. It takes an exclusive `flock` on `/tmp/fuku-<fingerprint>.lock`.
The lock is the only decision:

- the lock is free: the run owns the project. The guard keeps the file open for the life of the process
- another run holds it: `Check` refuses with `ErrInstanceAlreadyRunning`

The kernel releases the lock when the process exits, however it exits. The lock file is never removed.
An unlinked file would let a second run lock a new file under the same name.

The refusal goes to stderr and names where the other run answers:

```text
Error: fuku is already running for this project (socket /tmp/fuku-<fingerprint>.sock)
Run 'fuku logs' to follow it or stop that instance before starting another.
```

The socket probe only names the owner. When the other run's socket is not up yet, the refusal names `lock /tmp/fuku-<fingerprint>.lock`.
A file that cannot be opened or locked fails the start with `ErrFailedToLockProject`.

## Changing it

- the fingerprint, the socket name and the lock name are a compatibility contract. A running instance and a newer `fuku logs` must agree on them
- the lock decides. Never add a probe that refuses a run the lock admitted
- keep the lock file open on the guard. A dropped `*os.File` is closed by its finalizer, and that releases the lock
