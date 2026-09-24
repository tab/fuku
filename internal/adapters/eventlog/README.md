# adapters/eventlog

The event trail. Every bus message becomes one logfmt line.
It goes to the debug log under `BUS` and to the log clients as a line of the `fuku` service.

## How it works

`Recorder` is an optional subscription named `eventlog`. For each message:

1. `SnapshotChanged` and `ServiceResourcesSampled` are skipped. They would repeat every event
2. `Formatter` renders the wire name of the type, then the payload as `key=value` fields
3. the line goes to `log.Debug` and to `logs.Hub.Broadcast("fuku", line)`

The wire name is the string of `contracts.MessageType`. It is frozen once shipped. `fuku logs` clients read these lines.

A payload that carries a `model.Service` writes named fields only: the ID and the name. A lifecycle event adds the tier of its `ServiceEvent`.
The runtime fields on `model.Service` belong to the registry. They are zero in a payload and never reach a line.

## Changing it

- a new event with a payload gets a case in `Format`. Name the fields with the `Field*` constants
- a service is written field by field. A payload that holds one never falls through to the `data` field
- an event the read model repeats goes into `unrecorded`, not into the formatter
- the recorder is best effort. It drops lines when it falls behind. It never blocks a publisher
