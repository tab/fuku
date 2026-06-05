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
- **Log Streaming** - Stream logs from running instances via `fuku logs`
- **REST API** - Control and monitor services via HTTP with token authentication
- **Update Notifications** - TUI highlights a hint next to the version footer when a newer GitHub release is available (cached 24h, opt out via `FUKU_UPDATER_DISABLED=1`)

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

# Stop services for a profile (kills processes in service directories)
fuku stop                       # Default profile
fuku stop core                  # Specific profile

# Stream logs from running instance (in separate terminal)
fuku logs                       # All services
fuku logs api auth              # Specific services
fuku l api db                   # Short alias

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

`enter` opens a read-only info panel for the selected service with three tabs (`config`, `env`, `health`): working directory, start command, tier, readiness probe (type/address/URL/pattern), per-service log outputs, watch globs and debounce, merged contents of the service's `.env` files (display only — values are not exported to the child process), PID, uptime, retry policy, and the current lifecycle state with how long it has held. The panel auto-fits the services list to the longest service name (capped at a medium width so a single very long name cannot keep stealing aside space — names beyond the cap are truncated in the services column) and donates the rest of the terminal width to the aside; on terminals too narrow to fit both, the panel stays hidden and the services list keeps the full width. `tab` and `shift+tab` cycle the aside tabs; `\` switches keyboard focus between the services list and the aside (the focused panel takes the highlighted border and routes navigation keys). `esc` closes the panel; if the panel is already closed, `esc` clears an active filter.

## Configuration

Generate a config template with `fuku init`, or create `fuku.yaml` manually in your project root (`fuku.yml` is also supported as a fallback when `fuku.yaml` is absent).

### Local Overrides

Create `fuku.override.yaml` (or `fuku.override.yml`) next to your base config for local customizations that won't be committed:

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

Override merges are applied automatically when using default config discovery.
Explicit `--config` skips override loading. Maps are deep-merged, arrays are concatenated, and setting a key to `null` removes it.

### Excluding Services

Use the top-level `exclude` list to skip services at discovery time without mutating the service catalog.
Excluded services keep their definitions in `services:` (so profile lists referencing them still pass config validation) and `fuku run` will silently drop them at startup.
This is the recommended way to disable services locally via `fuku.override.yaml`.

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
  listen: "127.0.0.1:9876"
  auth:
    token: "dev-token"
```

For the full configuration reference, examples, and advanced patterns see the [documentation](https://getfuku.sh/docs/configuration/).

## Documentation

Full documentation is available at **[getfuku.sh](https://getfuku.sh)**:

- [Getting Started](https://getfuku.sh/docs/getting-started/) - First steps with fuku
- [Configuration](https://getfuku.sh/docs/configuration/) - All config options explained
- [CLI Reference](https://getfuku.sh/docs/cli/) - Commands, flags, and aliases
- [REST API](https://getfuku.sh/docs/api/) - HTTP endpoints for service control
- [Examples](https://getfuku.sh/docs/examples/) - Real-world configuration patterns
- [Troubleshooting](https://getfuku.sh/docs/troubleshooting/) - Common issues and solutions

## IDE Plugins

A [JetBrains plugin](https://getfuku.sh/plugins/jetbrains/) is available for GoLand, IntelliJ IDEA, WebStorm, and all JetBrains IDEs.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed architectural patterns and design decisions.

## Development

```bash
make fmt        # Format code
make vet        # Run go vet
make lint       # Run golangci-lint
make lint:fix   # Run golangci-lint with --fix
make test       # Run unit tests
make test:race  # Run tests with race detector
make build      # Build binary
make test:e2e   # Run e2e tests (requires build)
make coverage   # Generate coverage report
```

Verification loop:

```bash
make vet && make lint && make test && make build && make test:e2e && make test:race
```

## Privacy & Telemetry

Official release binaries include [Sentry](https://sentry.io) error tracking to help identify and fix bugs. This is completely transparent and can be disabled.

- Set `FUKU_TELEMETRY_DISABLED=1` to opt out
- Build from source to disable telemetry entirely

See [Privacy & Telemetry](https://getfuku.sh/docs/privacy/) for full details on what is and isn't collected.

## Update Notifications

On TUI startup, fuku checks the GitHub releases API for a newer version and renders a highlighted hint next to the version footer when an update is available (e.g. `v0.19.1 - ↑ v0.20.0`, with the available version in coral). The result is cached for 24 hours; network failures are silent.

- Set `FUKU_UPDATER_DISABLED=1` to disable the check entirely

## About the Name

The name fuku (福) means "good fortune" in Japanese. Inspired by jazz pianist Ryo Fukui, reflecting the tool's focus on orchestration and harmony.

## License

Distributed under the MIT License. See `LICENSE` for more information.
