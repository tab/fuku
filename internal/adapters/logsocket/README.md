# adapters/logsocket

The Unix socket transport of `fuku logs`: discovery, the wire frames, the server and the client.
Replay, filtering and history rules live in `app/logs`.

## The wire

One JSON object per line.

1. the client sends `subscribe {services, tail, noFollow}`
2. the server answers `status {version, instance, fingerprint, profile, services, pid, tail, noFollow}`
3. the server sends one `log {service, message}` per line until the queue closes or the client leaves

The status frame echoes the replay options on purpose. A bounded read (`--tail`, `--no-follow`) needs an acknowledgement.
If the first frame is not a status frame with the exact options requested, the client fails with `ErrBoundedReadNotSupported`.
An older server ignored the options and would stream forever.
`pid` is the server's process ID, which `fuku stop` signals. An older server omits it.

## The server

The server is a producer of the run composition.
It is listed before the services runtime, so it stops after the runtime's final events.

`run`:

1. waits on `registry.WaitResolved`
2. reads one snapshot for the profile and the service names
3. removes the stale sockets of other projects in the directory. Each is dialled. The ones that refuse are removed
4. binds the project socket at `instance.SocketPath(instance.SocketDir, fingerprint)`. A file already there is dialled once.
   It is removed unless it answers

A socket that answers means another instance owns the project. A bind failure is logged.
The run continues without the server. `Bound` waits for the bind attempt and returns its failure.
A detached start waits on it, so it never reports success without a socket.

Each connection reads one subscribe frame. A non-positive tail is rejected.
The server answers with the status frame, takes a hub subscription and pumps its lines. Each write is bounded by 5s.
A closed connection unsubscribes.

`Stop` cancels the accept loop, closes the listener and every connection, waits for them and removes the socket file.
It does this even when its context ends before `run` returns. It then returns the context error.
A bind still in flight at that moment lands after the stop. It stays open until the process exits.
The next run removes its socket file.

## The client

- `Connect` finds the socket for the project fingerprint and dials it. No socket is `ErrNoInstanceRunning`
- `Subscribe` sends the frame
- `Stream` decodes frames into `logs.Handler` calls until EOF or the context ends. A frame it cannot decode is skipped
- `Status` reads the status frame and disconnects. A socket that refuses the dial is `ErrNoInstanceRunning`, as after a `SIGKILL`

## Changing it

- a wire change is a compatibility change.
  A client and a server of different versions meet whenever a user upgrades fuku while a profile runs. Add fields.
  Never rename or remove one. Echo a new replay option in the status frame
- the server decides nothing about replay. It passes the request to the hub and pumps what comes back
