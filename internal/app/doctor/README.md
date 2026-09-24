# app/doctor

The rules of `fuku doctor`. It picks the checks, decides their severity and builds the report.
It sees the machine through three observer interfaces: `Environment`, `Filesystem`, `Runtime`. `adapters/diagnostics` implements them.
It resolves the active profile through `Profiles`. `app/profiles` implements it.

Nothing here touches the disk, the network or a socket. A check that needs a new fact gets a new observer method.

## The report

`Run` builds five sections in a fixed order:

1. Environment: `system`, `runtime`, `install`
2. Configuration: `config.file`, `config.override`, `config.validate`, `config.settings`
3. Services: `services.directories`, `services.dotenv`, `services.readiness`
4. Topology: `topology.tiers`, `topology.profile`
5. Runtime: `runtime.instance`, `runtime.sockets`, `runtime.ports`

Every run emits every row. A check that cannot run is skipped, never left out.

The config arrives as `model.Config`. A load failure sits in `Error`. Doctor reports it:

- a read or parse failure fails `config.file`
- `ErrInvalidConfig` fails `config.validate`. `config.file` stays ok
- a config that did not load skips `config.settings`, the services and topology sections and `runtime.ports`
- a profile that does not resolve fails `topology.profile`. The services section and `runtime.ports` are skipped

## Severity

Five levels: `ok`, `idle`, `note`, `warn`, `fail`. `idle` means "nothing to check".
A skipped check is `idle` with the reason in its summary.

`Overall` is the worst severity in the report. `Report.Tally` in `model` counts them. `Notes` lists everything but `ok` and `idle`.

`cli` maps the report to the exit code: 0, 2 when any check failed, 3 when rendering failed. A `warn` exits 0.

## Using it

`cli.Doctor` calls `Run` and hands the report to a `Renderer`.
`bootstrap/modules/doctor.go` picks one: the styled report, the summary or JSON. The JSON carries `schemaVersion` 1.

## Changing it

- a new check is one function in the file of its section, one `CheckID` in `model` and one row in the section list.
  Give it a remediation when it can fail
- a new fact about the machine is a method on an observer interface and its implementation in `diagnostics`. Never an `os` call here
- the JSON output is public. A changed shape bumps `schemaVersion`
