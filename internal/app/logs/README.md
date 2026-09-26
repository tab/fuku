# app/logs

The rules of the log pipeline: the bounded history, the client queues, the replay-to-live order, drop accounting and the session behind `fuku logs`.

Raw child output never touches the bus. The process adapter writes each line into the hub.
The hub fans it out to the socket server's subscriptions.

Two promises hold. Attaching a client never duplicates or loses a line. A slow client never blocks a running service.

## The hub

- `Broadcast` records the line in the ring. It offers the line to every subscription that follows the service. Both under one lock
- `Subscribe` replays the matching history into a new queue. It registers the subscription under the same lock.
  So a line that arrives during the attachment lands in the replay or in the live stream. Never both, never neither
- an empty service list follows everything
- `Tail` keeps only the newest lines of the replay
- `NoFollow` closes the queue after the replay and registers nothing

A queue holds `Buffer + History` lines, so a replay never drops. A live line that finds the queue full is dropped and counted.
`Unsubscribe` closes the queue and logs the count once.

Three interfaces bind to `*logs.Hub` in `bootstrap/modules/runtime.go`:

- `process.LogSink`: the child streams
- `eventlog.Broadcaster`: bus events, as lines of the `fuku` service
- `logsocket.Hub`: the clients

## The session

`Session.Run(ctx, Request)` connects the client, subscribes and streams.
It checks the status frame against the requested profile first. Another profile is `ErrProfileMismatch`.
A missing instance is `ErrNoInstanceRunning`, wrapped with the project.
Every failure is returned, not logged. The CLI maps any error to exit 1. The lifecycle prints it once.

The session knows no socket. `logsocket.Client` implements `Client`.
Two views implement `View`: `cli.LogView` for `--no-ui` and `tui.LogView` with the banner. `bootstrap/modules/logs.go` picks one.

## Changing it

- keep the replay and the registration under one lock. Splitting them reopens the gap that duplicates or loses a line
- a new replay option is a field on `model.ReplayOptions`. Apply it in `history.replay` or `Subscribe`.
  The socket server must echo it back, so a client can tell an older server ignored it
