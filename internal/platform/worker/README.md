# platform/worker

A bounded worker pool. It caps how many goroutines run a kind of work at once.

## How it works

`Pool` wraps a buffered channel sized to `Options.Workers`. `Acquire` takes a slot or returns the context's error
without taking one. `Release` frees a slot.

## Using it

`bootstrap/modules` binds the one `*Pool` to `services.Pool` and `process.Pool`, so one pool bounds both.

## Changing it

- `Release` must run exactly once per successful `Acquire`.
  A missing `Release` leaks one slot for the rest of the run.
  An extra `Release` frees a slot another caller holds. Then more than `Workers` can run at once.
  On an idle pool an extra `Release` blocks until the next `Acquire`
