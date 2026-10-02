# bootstrap/lifecycle

The application lifetime. A composition hands the coordinator its participants.
The coordinator starts them in order and stops them in reverse. The arbiter turns the first terminal outcome into the exit code.
`Run` starts and stops the Fx container.

The order lives in explicit slices, not in Fx value groups. Fx shuffles value groups.
The order matters: a consumer must hold its subscription before any producer publishes.

## Participants

Four roles. A package satisfies one structurally. No package imports this one.

- `Guard`: `Check` refuses the run. Today that is the single-instance check
- `Consumer`: `Subscribe` registers a bus subscription and returns. `Drain` waits until its queue is empty
- `Producer`: `Start` acquires what `Stop` releases. A goroutine, a listener, a watcher.
  `Stop` is empty when `Start` only publishes once, like `Announcer`
- `Command`: `Run` is the one command of the composition. Its return code is the exit code. Its error is the cause `Run` prints

A package can be both a consumer and a producer, like `services.Runtime`. It appears in both slices.

## Start and stop

Start: guard, telemetry, consumers in order, producers in order, command on its own goroutine.
`Start` creates the run context before the guard. The constructor creates nothing to cancel.
A failed stage unwinds the producers started so far and cancels the context.

`Run` traps `SIGINT` and `SIGTERM` before the start. A signal during the start runs the stop once the start returns.

Stop: `SignalReceived` is published if an OS signal ended the run. Producers stop, newest first.
Consumers drain in order, twice, because a handler may publish during the first pass. The first failed drain ends both passes.
The context is cancelled. The command is joined.
Telemetry stops next. It flushes Sentry, so the shutdown metrics and spans leave with it. The bus closes last, through `Closer`.
A failed start stops telemetry and closes the bus on its way out. The cancelled context has ended every subscription.

## Outcome

`Arbiter` records the first terminal cause. Later causes cannot overwrite it.

| Cause                                                          | Exit                                                                          |
| -------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| the command returned (`decide`)                                | the command's code, with `Error: <cause>` on stderr when it returned an error |
| a runtime failure (`Fail` from services or the bus) | 1, with `Error: <cause>` on stderr after the stop                             |
| an OS signal (`observe` from `Run`)                            | the signal code (0 unless the composition sets it), unless a failure came first |
| an Fx start failure                                            | 1, with the root cause                                                        |

`Run` prints the cause after `app.Stop`, once the TUI has released the terminal. It is the one report of a command failure.
A failed stop exits 1 and prints its error after the cause, so an overrun never hides why the run ended.
A constructor failure prints its own error, not the dependency chain that led to it.
A command returns its failure and does not log or print it. `ErrInstanceAlreadyRunning` prints nothing. The guard already did.

## Changing it

- a new participant goes into a composition's slices in `bootstrap/modules`, at the position its guarantee needs. Never into an Fx value group
- a package never registers an Fx hook of its own. The coordinator is the one Fx hook
- telemetry is not a producer. Producers stop before the consumers drain, but the collector and the tracer record during the drain.
  So telemetry starts after the guard and stops after the command is joined
