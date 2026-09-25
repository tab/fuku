# adapters/process

Child processes: launch a service command, track its handle, tee its streams, terminate it, and kill the orphans a previous run left behind.
When to launch, whether to retry and what an exit means belong to `app/services`. This package executes.

`Factory` is the one permitted "factory". Fx constructs it once. It creates a handle per launch.

## One child

`Factory.Start` runs the shell command in its own process group. It records the handle under the tracker lock in the same step.
So no window exists in which a live child is untracked.

Each stream goes to a stream writer that exec copies the child's output into:

- the writer passes every chunk to the handle's reader. Readiness reads it
- it also logs each line under the service and broadcasts it to the `LogSink`.
  The service's `logs.output` says which streams
- lines longer than 4 MiB are truncated in the broadcast

`wait` reaps the child, closes both writers and then closes `Done()`.
Exec stops writing before `Wait` returns, so the last line is never lost.

A descendant that inherited the streams can keep them open after the child exits.
`WaitDelay` bounds that: exec closes the pipes 5s after the exit.
The resulting `ErrWaitDelay` counts as a normal exit.

`Terminate` runs once per handle. Every later call shares the first result:

- the child already exited: nothing to do
- otherwise SIGTERM goes to the process group
- SIGKILL follows when the child has not exited within 5s

## The tracker

The tracker keeps the live handle per service, in start order:

- `Detach` marks a child that is being stopped on purpose. Its exit is expected
- `Untrack(id, proc)` forgets a handle only when it is the tracked one. It reports whether the handle was still active.
  `app/services` uses that to tell a crash from an intended stop
- `Reverse` lists every tracked child newest first, for the shutdown

## Preflight

`Cleanup(ctx, dirs)`:

1. publishes `PreflightStarted` with the service names
2. scans the running processes
3. matches each one's working directory to a service directory
4. kills every match within the worker bound: SIGTERM, SIGKILL after 2s
5. publishes `PreflightKilled` per process and `PreflightComplete`.
   `Killed` is the number of processes it went after. A failed kill still counts
6. returns the scan error. The caller logs it and continues

`app/services` decides the scope: the directories of the resolved profile.
It runs for `fuku run` before the first tier and for `fuku stop`.

## Changing it

- keep start-and-track under one lock and `Untrack` behind the identity check. Both exist so a stale exit is never reported against a newer child
- raw output stays off the bus. A line goes to the application log and to the `LogSink`. Nowhere else
