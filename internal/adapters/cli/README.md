# adapters/cli

The command line: cobra parsing, and every command that runs without the TUI.

`Parse` turns the arguments into one `Options` value: the command type, the profile, the config path and the flags of `logs` and `doctor`.
`bootstrap.Run` reads it and picks the composition. This package decides nothing about services.
It imports no `tui`. The composition selects the view.

## The commands

Each command is the `Command` participant of its composition. Its `Run` returns the exit code and the error the arbiter records.

- `Run` waits for the services runtime of `run --no-ui`. A failed run exits 1. A cancelled run exits 0
- `Stop` asks the cleaner to kill the processes of the profile
- `Logs` runs the log session of `fuku logs` until the stream ends
- `Doctor` runs the checks and writes the report through the selected renderer. A failed check exits 2. A report that cannot be written exits 3. A warning still exits 0
- `Init`, `Help` and `Version` write to their injected stdout and need no config

`Announcer` is a producer of every composition. Its `Start` publishes `CommandStarted`, so telemetry counts each command.

## Output

`LogView` prints the log stream as bare lines, without a banner. `bootstrap/modules/logs.go` picks it for `--no-ui`.
`JSON` renders the doctor report for machines. The styled report and the summary live in `tui`.

## Using it

`bootstrap.Run` calls `Parse`, then `ChangeToConfigDir`, so the project resolves from the directory of an explicit `--config` file.
The compositions bind the consumer interfaces: `Runtime` and `Cleaner` to the services core, `Session` to the log session
and `Checker` to the doctor runner. `LogView` takes `terminal.Log` as its `Formatter` directly. No binding exists for it.
The doctor `Renderer` is `JSON` here or a `tui` renderer, picked by `--json` and `--summary`.

## Changing it

- a new command is a struct with `Run(ctx) (int, error)`, a constructor in `module.go` and a row in `ARCHITECTURE.md`
- a flag is a `Flag` constant and a field on `Options`. A command that takes no config is listed in `standalone`, and `--config` with it is an error
- no rule about services is written here. A command calls a typed core method and maps the result to an exit code
