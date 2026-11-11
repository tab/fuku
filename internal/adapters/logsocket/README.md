# adapters/logsocket

The Unix socket transport of `fuku logs`: discovery, the wire frames, the server and the client.
Replay, filtering and history rules live in `app/logs`.

## The wire

One JSON object per line.

1. the client sends `subscribe {services, tail, noFollow}`
2. the server answers `status {version, instance, fingerprint, profile, services, api, pid, mode, tail, noFollow}`
3. the server sends one `log {service, message}` per line until the queue closes or the client leaves

`fuku stop` sends `stop` as the first frame instead. The server calls `Control.StopAll`, the same stop the TUI's `q` makes.
The server echoes `stop` once the core accepted it, then closes the connection. A refusal closes it without the echo.
So does an older server, which expects `subscribe`.

The status frame echoes the replay options on purpose, because a bounded read (`--tail`, `--no-follow`) needs an acknowledgement.
An older server ignored the options and would stream forever.
If the first frame is not a status frame with the exact options requested, the client fails with `ErrBoundedReadNotSupported`.

- `pid` is the server's process ID. `fuku stop` waits on it and signals it when the stop frame is not echoed
- `api` is the bound REST API address, read from the registry snapshot when the frame is written. It is omitted when the API is off
- `mode` is how the run holds its terminal: `tui`, `foreground` or `detached`. The composition projects it as `Options.Mode`

An older server omits `pid`, `api` and `mode`. A client treats a missing `api` or `mode` as unknown.

## The server

The server is a producer of the run composition. It is listed before the services runtime, so it stops after the runtime's final events.

`run`:

1. waits on `registry.WaitResolved`
2. reads one snapshot for the profile and the service names
3. dials the stale sockets of other projects in the directory and removes the ones that refuse
4. dials a file already at the project socket, `instance.SocketPath(instance.SocketDir, fingerprint)`, and removes it unless it answers
5. binds a socket inside a fresh private directory beside it, the socket path plus `.d`, with mode `0700`.
   A directory a crashed start left there is removed first. No other user can reach the socket inside, whatever the umask
6. sets the socket to mode `0600` and renames it to the project socket path. Only the user who runs fuku can connect.
   The private directory is removed on every outcome. A failed step closes the listener and is a bind failure

The private directory never matches the sweep's `fuku-*.sock` pattern, so the sweep of another project never touches it.
The single-instance guard holds the project lock, so no other start of the project owns the directory at that moment.

A socket that answers means another instance owns the project. A bind failure is logged, and the run continues without the server.
`Bound` waits for the bind attempt and returns its failure. A detached start waits on it, so it never reports success without a socket.

Each connection reads one frame. A subscribe frame with a non-positive tail is rejected.
Otherwise the server answers with the status frame, takes a hub subscription and pumps its lines. Each write is bounded by 5s.

`Stop` cancels the accept loop, closes the listener and every connection, waits for them and removes the socket file.
It does this even when its context ends before `run` returns, and then returns the context error.
A bind still in flight at that moment lands after the stop and stays open until the process exits.
The next run removes its socket file.

## The client

- `Connect` finds the socket for the project fingerprint and dials it. No socket is `ErrNoInstanceRunning`.
  A lookup that fails for another reason, such as a sandbox that denies it, is an error, never no instance
- `Stream` decodes frames into `logs.Handler` calls until EOF or the context ends. It skips a frame it cannot decode
- `RequestStop` sends the stop frame and fails unless the echo comes back within 2s
- `Status` reads the status frame and disconnects. A socket that refuses the dial is `ErrNoInstanceRunning`, as after a `SIGKILL`.
  Any other failure is an error, so `fuku stop` kills nothing under a fuku it cannot reach
- `Remove` deletes the project socket a dead instance left behind and keeps one that still answers. `fuku stop` ends with it

## Changing it

- a wire change is a compatibility change.
  A client and a server of different versions meet whenever a user upgrades fuku while a profile runs. Add fields.
  Never rename or remove one. Echo a new replay option in the status frame
- the server decides nothing about replay. It passes the request to the hub and pumps what comes back
