# adapters/config

The YAML side of `fuku.yaml`: find the file, merge the override, parse, validate, apply defaults and read the tier order.
The result is the plain `model.Project` every other package uses.

No package outside this one and `bootstrap` sees the YAML structs. The format can change without touching a core package.

## Loading

`LoadPath(path)` never fails the caller. It returns `model.Config{Path, OverridePath, Project, Topology, Error}`:

- the run compositions refuse an `Error` in `bootstrap.Run`, before any container exists
- Doctor turns it into findings
- help, version and init never load

An explicit `--config` path is read as is. Without one, `fuku.yaml` then `fuku.yml` in the working directory is read.
`fuku.override.yaml` or `.yml` beside it is merged on top.

`loadEnv` reads `.env.<GO_ENV>.local`, `.env.<GO_ENV>` and `.env` in that order. The first file to set a variable wins.
`Telemetry` calls it first, so the commands that load no project read the env files too.
`GO_ENV` must come from the process environment. `loadEnv` reads it before any file, so a `GO_ENV` in `.env` does not select `.env.<GO_ENV>`.
The variables stay in fuku's own environment. Every child process inherits them.

The merge works node by node:

- mappings merge by key
- sequences are concatenated
- an explicit `null` deletes a key
- anchors and merge keys (`<<`) of the base are resolved, so an override may use them

The merged document is parsed like a plain file. The run sees its declaration order.

Defaults fill the service directory (the service name), `defaults.tier` and the readiness timeout (30s) and interval (500ms).
Normalizing trims and lowercases tier names and deduplicates the exclude list.
Validation covers the logging level, concurrency, retry, logs, server, profile references and every service's command, readiness, logs and watch.
It runs before the defaults. A service with no body (`api:` alone) fails it. Viper would drop that service silently.

A validation failure is `contracts.ErrInvalidConfig`. A read failure is `contracts.ErrFailedToReadConfig`.
A parse failure is the local `ErrFailedToParseConfig`.

## The projection

`Project` turns the config into `model.Project`: `Services` sorted by tier order then name, `Profiles` (`*` becomes `All`), `Exclude` and the run-wide settings.

A `model.Service` carries identity and configuration:

- the ID is a UUID generated here, never read from YAML
- the command defaults to `make run`
- the tier defaults to `default` when unset or unknown
- the readiness timeout and interval default to 30s and 500ms
- the watch debounce defaults to 500ms
- the log output defaults to `stdout` and `stderr` when the list is absent or empty
- an absent `env.files` list defaults to `.env`, `.env.local`, `.env.development` and `.env.development.local`. `Defaulted` marks it
- an explicit empty `env.files` list stays empty. It loads nothing

Profile resolution preserves the service's ID and configuration.
Starts, stops and restarts use that same ID for the loaded project.

`bootstrap/modules` projects each package's `options.go` from `model.Project`.
Technical constants (socket paths, probe timeouts, port retries) live in the package that owns the rule. Never here.

## Changing it

- a new setting is a YAML field, a default or validation rule here and a field on `model.Project`.
  The package that reads it projects the field into its `options.go`. It never sees the YAML
- a default belongs here. A model value is complete when a package receives it
- the topology and the defaults resolve a service's tier through `serviceTier`. Change the rule there.
  Two rules would put a service in one tier and the tier order in another
