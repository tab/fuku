# adapters/readiness

The probes behind `readiness:`. `app/services` decides when to probe and what a failure means. This package only asks the question.

## The probes

`Check(ctx, readiness, proc)` runs one probe. On success it publishes `ReadinessComplete` with the type and the duration.

- `http`: `GET` the URL until a `2xx`. One request may run until `timeout`
- `tcp`: dial the address until it accepts
- `log`: scan stdout and stderr until a line matches the regex. A line over 4 MiB ends the scan of its stream

A probe ends on the first of: success, `ErrReadinessTimeout` after `timeout`, the context ending, or the child exiting (`ErrProcessExited`).
`http` and `tcp` pause `interval` between two attempts.

The streams a `log` probe scans are copies the child waits on. A stream is closed when its scan ends. Both are closed when the probe returns.
So the child's output flows on to the log once the probe stops, whatever stopped it.

`ProbePort(readiness)` is the pre-launch question: does the address already answer? One dial, 100ms.
`diagnostics` asks the same probe for the doctor. Keep one.
A `log` probe has no address and is never in use.

## Changing it

- the timeout and the interval come from `model.Readiness`. `adapters/config` fills the defaults. No default here
- a new probe type is a `model.Readiness*` constant, a case in `Check` and in `extractAddress`, and the field check in `app/doctor`
- keep the exit watch. A probe that outlives its child would report a timeout instead of the exit
- a `log` probe closes both streams on every return, the invalid pattern included. An open stream nobody reads stalls the child
