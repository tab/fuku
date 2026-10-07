# fuku

[![CI](https://github.com/tab/fuku/actions/workflows/master.yaml/badge.svg)](https://github.com/tab/fuku/actions/workflows/master.yaml)
[![codecov](https://codecov.io/github/tab/fuku/branch/master/graph/badge.svg?token=H1PA2DYMIZ)](https://codecov.io/github/tab/fuku)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**fuku** is a lightweight CLI orchestrator for running and managing multiple local services in development environments.

![screenshot](assets/demo.gif)

## Features

- **Interactive TUI** - Real-time service monitoring with status, readiness timeline, CPU, memory, and uptime
- **Service Orchestration** - Tier-based startup ordering
- **Service Control** - Start, stop, and restart services interactively
- **Graceful Shutdown** - SIGTERM with timeout before force kill
- **Profile Support** - Group services for batch operations
- **Readiness Checks** - HTTP, TCP, and log-pattern based health checks
- **Pre-flight Cleanup** - Automatic detection and termination of orphaned processes before starting services
- **Hot-Reload** - Automatic service restart on file changes
- **Single-Instance Guard** - A second `fuku run` of the same project is refused while the first one holds the project lock.
  A fuku older than this release running the same project holds no lock, so it is not detected during the upgrade
- **Detached Mode** - `fuku run -d` returns once every service is running, for scripts and AI agents
- **Log Streaming** - Stream logs from running instances via `fuku logs`
- **Diagnostics** - Check your config, environment, topology, and runtime with `fuku doctor`
- **REST API** - Control and monitor services via HTTP on `127.0.0.1:3858`, with an optional token

## Installation

### Homebrew

```bash
brew install tab/apps/fuku
```

### Install Script

```bash
curl -fsSL https://getfuku.sh/install.sh | sh
```

### Build from Source

```bash
git clone git@github.com:tab/fuku.git
cd fuku
go build -o cmd/fuku cmd/main.go
sudo ln -sf $(pwd)/cmd/fuku /usr/local/bin/fuku
```

## Quick Start

```bash
# Generate config file
fuku init                       # Creates fuku.yaml template
fuku i                          # Short alias

# Run with TUI (default profile)
fuku

# Run with specified profile without TUI
fuku run core --no-ui
fuku --no-ui run core           # Flags work in any position

# Use short aliases
fuku r core                     # Same as 'fuku run core'

# Run in the background and return once every service is running
fuku run -d                     # Default profile
fuku run core -d                # Specific profile, plain lines when piped or with --no-ui

# Stop the running fuku gracefully, then kill what is left in the service directories
fuku stop                       # Default profile
fuku stop core                  # Specific profile

# Stream logs from running instance (in separate terminal)
fuku logs                       # All services
fuku logs api auth              # Specific services
fuku logs api --profile core    # Filter by profile
fuku l api db                   # Short alias

# Bounded log read for scripts and agents
fuku logs api --tail 100        # Replay at most the newest 100 buffered messages, then follow
fuku logs api --no-follow       # Replay the buffered messages and exit
fuku logs api --no-ui --tail 100 --no-follow  # At most 100 messages, no panel, then exit

# Diagnose configuration, environment, and runtime issues
fuku doctor                     # Default profile
fuku doctor core                # Specific profile
fuku doctor --summary           # Compact one-line-per-check report
fuku doctor --json              # Machine-readable report (exit 2 on any failure)

# Use custom config file
fuku --config path/to/fuku.yaml run core
fuku -c custom.yaml run core

# Show help
fuku help                       # or --help, -h

# Show version
fuku version                    # or --version, -v
```

### TUI Controls

```
↑/↓ or k/j       Navigate services (or scroll the focused panel)
pgup/pgdn        Scroll the focused panel
home/end         Jump to start/end of the focused panel
enter            Open service info aside (toggles closed when already open)
tab / shift+tab  Cycle aside tabs forward / backward
\                Toggle focus between the services list and the aside
r                Restart selected service
ctrl+r           Restart all failed services
s                Stop/start selected service
/                Filter services by name
esc              Close service info aside, or clear filter when it is already closed
q                Quit (stops all services)
```

`enter` opens a read-only panel for the selected service. It has three tabs:

- `config`: directory, command, tier, readiness probe, log outputs, watch globs and debounce
- `env`: the merged `.env` files of the service. Display only. Nothing is exported to the child process
- `health`: PID, uptime, retry policy, the current state and how long it has held

The panel hides on a terminal too narrow for both columns.

## Configuration

Run `fuku init` for a template, or create `fuku.yaml` in your project root. `fuku.yml` works too.

### Local Overrides

Create `fuku.override.yaml` (or `.yml`) next to your config for local changes you do not commit:

```yaml
# fuku.override.yaml — typically .gitignored
services:
  api:
    command: "dlv debug ./cmd/main.go"  # use debugger locally
    watch:
      include: ["*.templ"]              # appended to base includes
  debug-tool:
    dir: tools/debug                    # add a local-only service

exclude:
  - heavy-worker                        # skip this service locally without editing profiles

logging:
  level: debug
```

The override is merged when fuku finds the config itself. An explicit `--config` skips it. Maps are deep-merged.
Arrays are concatenated. A key set to `null` is removed.

### Excluding Services

The top-level `exclude` list skips services at startup.
They keep their definitions in `services:`, so profiles that name them still validate.
This is the way to disable a service locally in `fuku.override.yaml`.

```yaml
exclude:
  - heavy-worker
```

See the [documentation](https://getfuku.sh/docs/configuration/) for full details.

### Example Configuration

```yaml
version: 1

services:
  auth:
    dir: auth
    tier: foundation
    command: go run cmd/main.go
    readiness:
      type: http
      url: http://localhost:8081/health
      timeout: 30s

  backend:
    dir: backend
    tier: platform
    readiness:
      type: http
      url: http://localhost:8080/health
      timeout: 30s

  web:
    dir: frontend
    tier: edge
    command: npm run dev

profiles:
  default: "*"
  backend: [auth, backend]

logging:
  format: console
  level: info

server:
  listen: "127.0.0.1:3858"
  auth:
    token: "dev-token"
```

The REST API runs on `127.0.0.1:3858` by default. The `server` block is optional.
A token is optional. Set `server.auth.token` to require `Authorization: Bearer <token>`.
The API binds a loopback address only and answers `403` to any request with an `Origin` header, which turns away a browser.
Set `server.listen` to `""` or `none`, or `FUKU_API_DISABLED=1`, to turn it off.

The full reference is in the [documentation](https://getfuku.sh/docs/configuration/).

## Documentation

Full documentation is available at **[getfuku.sh](https://getfuku.sh)**:

- [Getting Started](https://getfuku.sh/docs/getting-started/) - First steps with fuku
- [Configuration](https://getfuku.sh/docs/configuration/) - All config options explained
- [CLI Reference](https://getfuku.sh/docs/cli/) - Commands, flags, and aliases
- [REST API](https://getfuku.sh/docs/api/) - HTTP endpoints for service control
- [Examples](https://getfuku.sh/docs/examples/) - Real-world configuration patterns
- [Troubleshooting](https://getfuku.sh/docs/troubleshooting/) - Common issues and solutions

## Plugins

A [JetBrains plugin](https://getfuku.sh/plugins/jetbrains/) is available for GoLand, IntelliJ IDEA, WebStorm, and all JetBrains IDEs.

An [AI agents plugin](https://getfuku.sh/plugins/agents/) lets Claude Code and Codex run, inspect, debug and stop local services with fuku.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for the layers, the features and how they are wired.

## Development

```bash
make check                   # Format, lint, vet, unit tests
make build && make test:e2e  # E2E tests against the built binary
make test:race               # Unit tests with the race detector
make docs                    # Check the links in ARCHITECTURE.md and the package READMEs
make lint:plugin             # Lint the JetBrains plugin
```

## Privacy & Telemetry

Release binaries include [Sentry](https://sentry.io) error tracking. Set `FUKU_TELEMETRY_DISABLED=1` to opt out, or build from source.
See [Privacy & Telemetry](https://getfuku.sh/docs/privacy/) for what is collected.

## Update Notifications

Under the TUI, `fuku run` checks GitHub for a newer release once a day. The TUI shows it next to the version (`v0.19.1 - ↑ v0.20.0`).
Network failures are silent. Set `FUKU_UPDATER_DISABLED=1` to turn the check off.

## About the Name

The name fuku (福) means "good fortune" in Japanese. Inspired by jazz pianist Ryo Fukui, reflecting the tool's focus on orchestration and harmony.

## License

Distributed under the MIT License. See `LICENSE` for more information.
