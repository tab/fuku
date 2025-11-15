# fuku

**fuku** is a lightweight CLI orchestrator for running and managing multiple local services in development environments.

![screenshot](assets/screenshot.png)

## Features

- **Interactive TUI** - Real-time service monitoring with status, CPU, memory, and uptime
- **Service Orchestration** - Tier-based startup with dependency resolution
- **Service Control** - Start, stop, and restart services interactively
- **Graceful Shutdown** - SIGTERM with timeout before force kill
- **Profile Support** - Group services for batch operations
- **Readiness Checks** - HTTP and log-pattern based detection
- **Log Streaming** - Filter and view service logs in real-time

## Installation

```bash
git clone git@github.com:tab/fuku.git
cd fuku
go build -o fuku ./cmd
```

## Quick Start

```bash
# Run with TUI (default)
fuku --run=default

# Run without TUI
fuku --run=default --no-ui
```

### TUI Controls

```
↑/↓ or k/j  Navigate services
s           Stop/start service
r           Restart service
space       Toggle log subscription
l           Switch to logs view
q           Quit gracefully
ctrl+c      Force quit
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

profiles:
  default:
    include: [postgres, api, web]

  backend:
    include: [postgres, api]

logging:
  format: console
  level: info
```

### Service Requirements

Each service directory must have a Makefile with a `run` target:

```makefile
run:
	npm start
```

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed architectural patterns and design decisions.

## Development

### Building

```bash
# Build binary
go build -o fuku ./cmd

# Run tests
make test

# Run linter
make lint

# Run vet
make vet

# Format code
go fmt ./...

# Full validation
go fmt ./... && make lint && make vet && make test
```

## About the Name

The name fuku (福) means "good fortune" in Japanese. Inspired by jazz pianist Ryo Fukui, reflecting the tool's focus on orchestration and harmony.

## License

Distributed under the MIT License. See `LICENSE` for more information.
