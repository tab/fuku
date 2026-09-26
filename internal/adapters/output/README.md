# adapters/output

The application log writer. The slog handler writes every application log line into it, and it writes them to stdout.

## How it works

The handler emits one JSON object per line. `Writer` parses it and renders the console form through `terminal.Log`:
the `service` key names the line, `fuku` when there is none, and the `component` key becomes a bracketed prefix such as `[PROCESS]`.
With the JSON log format the line is written as it is. A line that is not JSON is written as it is too.

The writer starts disabled. The composition enables it for every command that writes to the terminal.
`fuku run` with the TUI keeps it off for the whole run, so no log line tears the view or follows it.
A failed run still prints `Error: <cause>` on stderr through the arbiter.
A line written while the writer is off is dropped, not buffered.

## Using it

`bootstrap/modules/runtime.go` creates the writer with `terminal.Log` as its `Formatter` and builds the logger on it.
`run.go` turns it off under the TUI. Nothing turns it back on.

## Changing it

- the writer formats nothing itself. A new look for a log line is a change in `terminal.Log`
- the keys it reads (`component`, `message`, `service`) are the ones `platform/logging` emits. Rename them together
- keep the switch and the writes under the mutex. The logger writes from many goroutines
