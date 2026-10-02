# adapters/tui

The Bubble Tea frontends: the services view of `fuku run`, the startup view of `fuku run -d`,
plus the inline doctor report and log view that render without a program.

The view owns interaction, animation and history. Every shared fact comes from the registry snapshot.
Every action goes through `Control`. The view never decides what a service may do.

Every style comes from `adapters/terminal`. No `lipgloss.NewStyle()` is called here.
`Program` resolves the injected theme in `Run` and hands it to the model. The model rebuilds it when the terminal reports its background color.
The inline doctor report and log view resolve the theme when they write. The log banner reads the terminal width then too.

## How messages reach the view

`Bridge` is a consumer of the view composition.
It subscribes to `SnapshotChanged`, the preflight events, `SignalReceived` and `UpdateAvailable`.
Its handler blocks until the program exists. A message published before the view attaches waits in the queue and is replayed in order.

`Program` is the command of the composition.
It builds the model, attaches the program to the bridge and runs it with `tea.WithoutSignalHandler()`. Fx owns the OS signals.
A signal reaches the view as its cancelled context.
The application log writer stays off for the whole run. A failed run still prints `Error: <cause>` on stderr through the arbiter.
The program's exit code is the exit code of the run, unless the arbiter recorded a failure first.

## The read model

The view keeps no copy of the registry. `Update` handles each message inside one `registry.Read`. `View` renders inside another.
Both set the model's `snapshot` inside the callback. `Update` clears it before it returns the model.
So the model Bubble Tea keeps never points into the registry. A read outside a callback panics instead of racing.

`serviceView` holds two kinds of fields and nothing else:

- view-only state the snapshot cannot hold: `Blink`, `Timeline`, `StartupSampled`, `StartupActive`
- the last seen value of each snapshot field the view compares to detect a transition: `Status` and `AttemptedAt`

A field the view only displays is never copied. It is read from the snapshot's `*model.Service`.

`applySnapshot` runs on every `SnapshotChanged` and on the one-second tick. The tick recovers a dropped notification:

- the first resolved snapshot builds the view state in tier order and leaves the loader alone, because a preflight event may arrive first
- then every service goes through `applyService`
- the stopped phase clears the loader and quits

`applyService` records what it saw. It runs effects only on a status change or a new attempt:

- `starting` opens the attempt and starts the loader
- `running` and `failed` settle the attempt and stop the loader
- `stopped` settles and keeps the loader while a restart is in flight
- `stopping` starts the loader
- `restarting` flags the restart and starts the loader

## Actions

- `s` starts a stopped or failed service, and stops a running one
- `r` restarts the selected service
- `ctrl+r` restarts every failed service
- `q` stops everything

A key handler reads the target's ID, name and `LifecycleAt` inside `Read` and returns a command. It never calls `Control` under the lock.
The command calls `Control`. It captures those three values and a `Control` method, never a `*model.Service` or the model.
It answers with `admissionMsg`, or with `stopAllMsg` for `q`. The view reacts to that answer:

- the loader starts only when the core admitted the action and the service's `LifecycleAt` is still the one the key saw.
  A later one means the action's events arrived first, and the snapshot already drives the loader
- `q` marks the view as shutting down only when the core accepted it
- a rejected action is logged at debug. A rejected quit is logged at warn

## Changing it

- a new fact the view shows is read from the snapshot inside `Read`. The view subscribes to no lifecycle event
- a new transition the view reacts to keeps the last seen value of the compared field in `serviceView`. Nothing else goes there
- a new action goes through `Control`. The view never checks eligibility
- a new style is a field on `terminal.Theme` or a `var` in `terminal/styles.go`

## The startup view

`Startup` implements `detach.View` for a detached start on a terminal. It draws one line per service, the way `docker compose up -d` does.
It runs inline and reads the terminal, so the replies to Bubble Tea's mode queries never leak into the shell.
In raw mode `Ctrl-C` is a key press. The model turns it into a `SIGINT` to its own process, the one abort path of the parent.
`Close` sends the final frame and waits for the program. The frame stays on screen above the summary.
