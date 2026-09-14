# Project-scoped sockets

## Goal

Give each project its own relay socket, so `fuku logs` and the JetBrains plugin reach the instance started from the
current project directory and two projects can run the same profile side by side.

## Context

The relay socket is `/tmp/fuku-<profile>.sock`.
The name carries the profile but not the project, so every project on the machine shares one socket per profile name.

When two projects run the same profile, the second instance finds the socket in use, logs a warning and continues
without log streaming.
`fuku logs` in the second project then connects to the first project's socket and shows the wrong logs.

Fuku already reports a project fingerprint in the relay status message and `GET /api/v1/live`, but does not use it
to locate the socket.
The JetBrains plugin builds the socket path from the profile it reads from `GET /api/v1/status`.

## Scope

### In

- Name the socket `/tmp/fuku-<fingerprint>.sock`, using the project fingerprint from the instance identity
- Bind the relay server to the project socket
- Make `fuku logs` connect to the current project's socket
- Turn `fuku logs --profile <name>` into a check that the running instance serves that profile
- Make the doctor runtime check look for the current project's socket
- Run `fuku doctor` inside an FX container so the runtime check receives the instance identity through injection
- Update the JetBrains plugin to read the fingerprint from `GET /api/v1/live` and connect to the project socket
- Make the single-instance guard dial the project socket, so a second `fuku run` of the same project is refused
  with or without the API (BL-004)
- Add focused unit and end-to-end tests
- Update `ARCHITECTURE.md` and the log-streaming docs

## Expected behavior

### Two projects, one profile name

```bash
cd ~/projects/some-project && fuku run --no-ui
cd ~/projects/another-project && fuku run --no-ui
```

Each instance listens on its own `/tmp/fuku-<fingerprint>.sock` and streams logs.
`fuku logs` in each directory shows only that project's logs.

### Log streaming

```bash
fuku logs
fuku logs api auth
```

The command computes the fingerprint of the current directory, connects to that socket and streams.

### Profile check

```bash
fuku logs api --profile core
```

A project has only one socket, so the flag names the profile the caller expects instead of selecting a socket.
When the running instance reports another profile in its status message, the command exits with code 1 before printing
any log line:

```text
fuku        | [LOGS] Fuku is running profile 'default', not 'core'
```

When the profiles match, or the flag is absent, streaming proceeds.

### Second run of one project

```bash
cd ~/projects/some-project && fuku run backend --no-ui
cd ~/projects/some-project && fuku run frontend --no-ui
```

The second command finds the project socket answering, prints the refusal to stderr and exits with code 1 before
its pre-flight cleanup can touch the first instance's services, whether or not `server.listen` is set:

```text
Error: fuku is already running for this project (socket /tmp/fuku-<fingerprint>.sock)
Run 'fuku logs' to follow it or stop that instance before starting another.
```

A stale socket file left by a crashed instance does not answer, so it does not block the next run.

### Acceptance criteria

- **AC1** – the relay server listens on `/tmp/fuku-<fingerprint>.sock` for the project it serves
- **AC2** – two projects running the same profile name each bind their own socket and stream their own logs
- **AC3** – `fuku logs` connects to the current project's socket and never to another project's
- **AC4** – `fuku logs --profile <name>` against an instance serving another profile exits with code 1 before any log
  line and names both profiles
- **AC5** – `fuku logs --profile <name>` against an instance serving that profile streams
- **AC6** – `fuku doctor` reports a running instance for the current project through the new path
- **AC7** – the JetBrains plugin connects to `/tmp/fuku-<fingerprint>.sock` using the fingerprint from
  `GET /api/v1/live`
- **AC8** – unit tests cover the socket path, the server bind, the logs profile check and the doctor check, an
  end-to-end test covers two projects sharing a profile name, and the plugin passes `make lint:plugin` and
  `make build:plugin`
- **AC9** – `ARCHITECTURE.md` and the log-streaming docs describe the project socket
- **AC10** – a second `fuku run` of a project whose socket answers exits with code 1 and names the socket, with the
  API enabled or disabled, while a stale socket file lets the run proceed

## Assumptions

- `instance.Identity` is already provided through FX to the relay server and can be injected into the logs screen
- The doctor checks tolerate a missing config, so the doctor container needs only the command options and the
  instance identity
- `internal/bootstrap` already dispatches the commands, so the doctor container belongs in `bootstrap.DoctorCLI`
- `fuku logs` hashes the directory it runs from, so a run from a subdirectory of the project finds no socket and
  names that directory in its error
- The plugin polls the API of the project it is configured for, so the fingerprint from `/live` belongs to the same
  instance as the socket

## Contracts

The socket path is `<SocketDir>/fuku-<fingerprint>.sock`.
The fingerprint is the first 16 hex characters of the SHA-256 of the symlink-resolved project directory, as
`instance.Fingerprint` computes it and as `GET /api/v1/live` and the relay status message report it.

`--profile` on `fuku logs` names the profile the caller expects the instance to serve.
A mismatch with the profile in the relay status message is an error; an omitted flag accepts any profile.

## Decisions

- Put only the fingerprint in the name because a project runs one instance and the profile is available from the status
  message; the guard enforces that rule through the socket, so two profiles of the same project can no longer run side
  by side
- Let the guard dial the socket before the API probe, because the socket exists in every run mode and identifies the
  project directly; the API probe stays for an instance whose relay server failed to bind
- Keep `--profile` as a check rather than removing it, so the documented flag keeps a clear meaning and nothing is
  silently ignored
- Let the plugin read the fingerprint from `/live` rather than hashing the project path itself, so one implementation
  of the fingerprint exists
- Move doctor into FX rather than calling `instance.NewInstance()` directly, because every dependency in fuku is
  injected through FX
- Accept the compatibility break with instances started by an older binary: few users run the REST API, developers do
  not keep an old instance running for long and restarting it after the upgrade is enough
