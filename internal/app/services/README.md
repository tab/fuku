# app/services

The service workflow: tier startup, readiness, retry, admission, watch restarts and shutdown. This package decides.
`adapters/process` launches and terminates. `adapters/readiness` probes. `adapters/watch` reports file changes.

Every frontend calls `Control`. `Control` asks one `Guard`. The guard owns every fact the answer depends on.
So the TUI, the REST API and the watcher cannot disagree about what a service may do.

## The run

`Runtime` is a consumer and a producer of the run composition:

- `Subscribe` registers the required `services` subscription: the four commands and `WatchTriggered`
- `Start` opens the run and starts the tiers on its own goroutine
- `Stop` cancels the run under the launch lock and waits. No child starts once a stop began

The run:

1. publish `PhaseChanged{startup}`
2. resolve the profile, publish `ProfileResolved`
3. run the preflight cleanup over the service directories
4. start the tiers in declaration order, one at a time
5. publish `PhaseChanged{running}` and wait for the context to end
6. publish `stopping`, stop the children newest first, publish `stopped`. The children of one tier stop together

`fuku stop` runs step 3 alone, through `Cleaner`. No run opens.

A run cancelled during startup returns `errStartupInterrupted`. A `StopAll` during startup is the same clean end. Any other failure reaches `Reporter.Fail`, which exits 1.

A start or restart cut short by the end of the run publishes `ServiceStopped`. Never `ServiceFailed`, and never nothing.
It applies while the service waits for a worker, runs its readiness check or waits out a retry backoff.
So every service the run touched ends in a terminal status.

One attempt at a service:

1. probe the port. An address in use fails before any child exists
2. launch under the run lock
3. publish `ServiceStarting{Attempt}`
4. run the readiness check
5. publish `ServiceReady` and start the exit watcher

The resolved `model.Service` carries the ID, tier and configuration through every attempt.
The launcher receives that one value, and lifecycle actions need no config lookup by name.

A child that never became ready is terminated before the backoff. `ServiceFailed` follows the last attempt.
Every start runs inside the worker bound.

## An action

1. the frontend calls `Control.Restart(id)`
2. `Guard.admit` checks the request and reserves the token
3. `Control` publishes `CommandRestartService` and returns the `Admission`
4. `Runtime` claims the token and runs the action on its own goroutine
5. the token is released when the action ends

`Guard.admit` checks, in one locked step:

1. the run is in startup or running and the profile is resolved, or `ErrNotAccepting`
2. the ID belongs to the profile, or `ErrServiceNotFound`
3. tier startup has dispatched the service at least once, or `ErrActionNotAllowed`
4. no token is held, or `ErrServiceBusy`
5. the action fits the live child: start needs none, stop needs one, restart needs nothing more. Otherwise `ErrActionNotAllowed`

`Toggle` serves the TUI's `s` key. It picks stop for a service with a live child and start for one without.
`Guard.admit` then checks that action like any other.

A rejected publish releases the token and returns the bus error.

An admitted stop marks the service as stopped on purpose. An admitted start or restart clears the mark, and so does tier startup.

The `Admission` carries the predicted status. The REST `202` body and the TUI loader use it. `202` means admitted, not done.
The lifecycle events say what happened.

A `WatchTriggered` restart takes the token with `reserve`. It is dropped when the token is held. A watch never queues.
It is also dropped for a service stopped on purpose, so a change that arrives after the stop never restarts it.
A failed service carries no mark. The next change restarts it.
`StopAll` closes admission and publishes `CommandStopAll`.

Once a service is ready, `watchForExit` waits on the child. A child stopped on purpose was detached first. Its exit is ignored.
An unexpected exit publishes `ServiceFailed{ErrUnexpectedExit}` for a watched service, so the next file change restarts it.
Any other service gets `ServiceStopped{Unexpected: true}`.

## Changing it

- admission reads the guard and the tracker only. Never the registry snapshot. A lagging read model must not authorize work
- the token is released on every path, including a rejected publish and a cancelled attempt
- a new action is a `contracts.Action`, an `allowed` rule, a predicted status and a command type in `Action.Command`. The frontends stay unchanged
