# bootstrap/modules

One composition per command. A composition is the `fx.Option` that `bootstrap.Run` hands to `fx.New`.
It stacks the blocks a command needs and binds each adapter to the interface its core package declares.
It projects each package's options and hands `lifecycle.Coordinator` its participants in order.
No rule lives here. A block that makes a decision belongs in the package that owns it.

`bootstrap.Run` supplies the build-time `SentryDSN` once to Fx.
`base` provides `telemetry.Options` from that fallback and the environment or project telemetry settings.
An environment DSN takes precedence, and the telemetry opt-out still applies.
`bootstrap.Run` reads the environment once, before any container exists. Help, version and init get it from `config.Telemetry`.
Doctor and the configured commands take it from the loaded project. A doctor whose config failed to load reads it again through `config.Telemetry`.

## How it works

The blocks are `var`s and functions in `base.go`, `runtime.go` and `run.go`. Each one adds to the one below it.

| Block          | Adds                                                                                                                            |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `base`         | the bus, the coordinator, the arbiter, telemetry, stdout and stderr. Every composition carries it                               |
| `standalone`   | the command, env telemetry, the announcer, a discard logger, a silent Fx. For the commands that load no project                 |
| `configured`   | the project, the command, the logger, the theme, the log hub, the event log                                                     |
| `processes`    | the process adapter, the worker pool, the profile resolver, the services package (`Cleaner` complete)                           |
| `profile`      | `processes` plus readiness and the services options, so `Runtime`, `Guard` and `Control` resolve                                |
| `runtime`      | `profile` plus the instance, the registry, the watcher, the sampler, the socket server, the update check                        |
| `api`          | the REST server. Only when `server.listen` is set                                                                               |
| `headless`     | `cli.Run` as the command                                                                                                        |
| `view`         | `tui.Program` as the command, the environment store and `tui.Bridge` as consumers                                               |

| Composition                        | Blocks                                                                                                   |
| ---------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `run.go`                           | `base`, `configured`, `runtime`; `api` with a listen address; `view`, or `headless` with `--no-ui`       |
| `stop.go`                          | `base`, `configured`, `processes`                                                                        |
| `logs.go`                          | `base`, `configured`, `instance`, `logsocket`; the inline view, or bare lines with `--no-ui`             |
| `doctor.go`                        | `base`, `standalone`, the theme, `instance`, `diagnostics`, `profiles`, `doctor`                         |
| `init.go`, `help.go`, `version.go` | `base`, `standalone`                                                                                     |

The participants of a command, in order:

- every command runs over `Announcer` as a producer, and `Collector` and `Tracer` as consumers when telemetry is on
- `configured` puts `Recorder` before them
- `run.go` adds the runtime around them. Consumers: `Runtime`, `Registry`, `Watcher`, the observers, then `Environment` and `Bridge` under the view
- Producers: `Announcer`, `Socket`, `Watcher`, `Runtime`, `Sampler`, `Checker`, then `Server` with a listen address
- the socket server and the watcher precede the runtime. They are open before a service becomes ready and stop after the runtime published its final events
- the guard is `instance.Guard`. The command is `tui.Program`, or `cli.Run` with `--no-ui`
- the stop budget is Fx's default plus one `process.ShutdownTimeout` per service, so a stop never outruns the children's shutdown

What the order guarantees is in [`bootstrap/lifecycle/README.md`](../lifecycle/README.md).

## Adding a package

- the package's `module.go` provides its constructors. It binds nothing
- the composition binds the concrete type to each consumer's interface: `func(c *github.Client) updater.ReleaseSource { return c }`
- it projects the package's `Options` from `model.Project` or `cli.Options`, and its `Logger` with the component name
- a lifecycle participant goes into the slices at the position its guarantee needs
- a type that must be absent from a command gets a case in `base_test.go`

The wiring rules are in [`CLAUDE.md`](../../../CLAUDE.md#dependency-injection-with-fx).

## Changing it

- `base_test.go` validates every composition once with `fx.ValidateApp` and proves an absent type per command.
  No package writes a `module_test.go`; the reason is in [`CLAUDE.md`](../../../CLAUDE.md#modulego-holds-wiring-only)
- `Test_Projections` pins what each composition projects. A new `Options` type gets a field there
- a block is unconditional. `Run` picks `view` or `headless` and `api`; `newLogsView` and `newDoctorRenderer` pick a view.
  No package checks the command type or a listen address
- the order in a participant slice is a guarantee. Fx value groups are shuffled and cannot carry it
- building the graph queries no terminal and loads no `.env` file. The theme is a function that detects the terminal on its first call, not at construction
