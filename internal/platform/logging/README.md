# platform/logging

The `slog.Handler` that writes one zerolog line per record.

## How it works

`Handle` writes the record's message and every attribute, its own and the ones `WithAttrs` carried in, as one zerolog event.
`WithGroup` prefixes later keys with `name.`. The configured level maps to the matching zerolog level. An unknown level reads as `info`.

## Using it

`bootstrap/modules` is the only caller. It builds one `Handler` from `Options{Level, Version}` and wraps it in the `*slog.Logger` the run shares.

## Changing it

- `internal/adapters/output` renders the `component`, `message` and `service` keys of the line this package emits.
  Rename or drop one only together with that reader
