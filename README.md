# fuku

[![CI](https://github.com/tab/fuku/actions/workflows/checks.yaml/badge.svg)](https://github.com/tab/fuku/actions/workflows/checks.yaml)
[![codecov](https://codecov.io/github/tab/shortly/graph/badge.svg?token=R8T7W6DQ9T)](https://codecov.io/github/tab/shortly)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**fuku** is a lightweight CLI orchestrator for running and managing multiple local services in development environments.

![screenshot](assets/demo.gif)

## Features

- **Interactive TUI** - Real-time service monitoring with status, CPU, memory, and uptime
- **Service Orchestration** - Tier-based startup ordering
- **Service Control** - Start, stop, and restart services interactively
- **Graceful Shutdown** - SIGTERM with timeout before force kill
- **Profile Support** - Group services for batch operations
- **Readiness Checks** - HTTP and log-pattern based detection
- **Hot-Reload** - Automatic service restart on file changes
- **Log Streaming** - Stream logs from running instances via `fuku logs`

## Installation

### Download from Releases

Download the latest binary from [GitHub Releases](https://github.com/tab/fuku/releases)

**macOS (Apple Silicon):**
```bash
cd ~/Downloads
tar -xzf fuku_v0.8.1_macos_arm64.tar.gz
sudo xattr -rd com.apple.quarantine ~/Downloads/fuku_v0.8.1_macos_arm64/fuku
sudo mv ~/Downloads/fuku_v0.8.1_macos_arm64/fuku /usr/local/bin/fuku
```

**macOS (Intel):**
```bash
cd ~/Downloads
tar -xzf fuku_v0.8.1_macos_amd64.tar.gz
sudo xattr -rd com.apple.quarantine ~/Downloads/fuku_v0.8.1_macos_amd64/fuku
sudo mv ~/Downloads/fuku_v0.8.1_macos_amd64/fuku /usr/local/bin/fuku
```

**Linux:**
```bash
cd ~/Downloads
tar -xzf fuku_v0.8.1_linux_amd64.tar.gz
sudo mv ~/Downloads/fuku_v0.8.1_linux_amd64/fuku /usr/local/bin/fuku
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
# Run with TUI (default profile)
fuku

# Run with specified profile without TUI
fuku run core --no-ui
fuku --no-ui run core           # Flags work in any position

# Use short aliases
fuku r core                     # Same as 'fuku run core'

# Stream logs from running instance (in separate terminal)
fuku logs                       # All services
fuku logs api auth              # Specific services
fuku logs --profile core api    # Filter by profile
fuku l api db                   # Short alias

# Show help
fuku help                       # or --help, -h

# Show version
fuku version                    # or --version, -v
```

### TUI Controls

```
↑/↓ or k/j       Navigate services
pgup/pgdn        Scroll viewport
home/end         Jump to start/end
r                Restart selected service
s                Stop/start selected service
q                Quit (stops all services)
```

## Configuration

Create `fuku.yaml` in your project root:

```yaml
version: 1

services:
  postgres:
    dir: ./infrastructure/postgres
    tier: foundation
    readiness:
      type: log
      pattern: "database system is ready"
      timeout: 30s

  api:
    dir: ./api
    tier: platform
    readiness:
      type: http
      url: http://localhost:8080/health
      timeout: 30s

  web:
    dir: ./frontend
    tier: edge
    profiles: [default]

defaults:
  profiles: [default]

profiles:
  default: "*"                    # All services
  backend: [postgres, api]        # Backend services only
  minimal: [api, postgres]        # Minimal set

concurrency:
  workers: 5                      # Max concurrent service starts

retry:
  attempts: 3                     # Max retry attempts
  backoff: 500ms                  # Initial backoff duration

logs:
  buffer: 100                     # Log streaming buffer size

logging:
  format: console
  level: info
```

### Tiers

Services are organized into tiers for startup ordering.
You can use any tier names you want - the startup order is determined by the first occurrence of each tier name in your `fuku.yaml` file.

Common tier naming pattern:
- **foundation** - Base infrastructure (databases, message queues)
- **platform** - Business logic services
- **edge** - Client-facing services

You can also use custom tier names like `infrastructure`, `middleware`, `api`, `frontend`, etc. The key points:
- Tier order is defined by first appearance in the YAML file
- Services within each tier are sorted alphabetically by name
- Services without a tier are placed in a `default` tier that runs last
- Tier names are case-insensitive and whitespace is trimmed

For example, if your YAML defines services with tiers in this order: `foundation` → `platform` → `edge`, services will start in that order, tier by tier.

### Readiness Checks

- **log** - Wait for pattern in service output
- **http** - Wait for HTTP endpoint to respond

### Service Requirements

Each service directory must have a Makefile with a `run` target:

```makefile
run:
	npm start
```

Check examples in the [examples](examples/bookstore) directory for reference.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed architectural patterns and design decisions.

## Development

### Tests and linters

```bash
# Run tests
make test

# Run linter
make lint

# Run vet
make vet

# Run coverage
make coverage

# Format code
go fmt ./...

# Full validation
make vet && make lint && make test
```

## About the Name

The name fuku (福) means "good fortune" in Japanese. Inspired by jazz pianist Ryo Fukui, reflecting the tool's focus on orchestration and harmony.

## License

Distributed under the MIT License. See `LICENSE` for more information.
