# fuku

[![CI](https://github.com/tab/fuku/actions/workflows/checks.yaml/badge.svg)](https://github.com/tab/fuku/actions/workflows/checks.yaml)
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
- **Log Streaming** - Filter and view service logs in real-time

## Installation

```bash
git clone git@github.com:tab/fuku.git
```

```bash
cd fuku
go build -o cmd/fuku cmd/main.go
sudo ln -s $(pwd)/cmd/fuku /usr/local/bin/fuku
```

## Quick Start

```bash
# Run with TUI (default)
fuku

# Run without TUI
fuku --run=default --no-ui

# Show help
fuku help

# Show version
fuku version
```

### TUI Controls (services)

```
↑/↓ or k/j       Navigate services
pgup/pgdn        Scroll viewport
home/end         Jump to start/end
r                Restart selected service
s                Stop/start selected service
space            Toggle logs for selected service
ctrl+a           Toggle all logs
tab              Switch to logs view
q                Quit
```

### TUI Controls (logs)

```
↑/↓ or k/j       Scroll logs
pgup/pgdn        Scroll viewport
home/end         Jump to start/end
a                Toggle autoscroll
ctrl+r           Clear logs
tab              Switch back to services view
q                Quit
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
