# fuku

[![CI](https://github.com/tab/fuku/actions/workflows/master.yaml/badge.svg)](https://github.com/tab/fuku/actions/workflows/master.yaml)
[![codecov](https://codecov.io/github/tab/fuku/branch/master/graph/badge.svg?token=H1PA2DYMIZ)](https://codecov.io/github/tab/fuku)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**fuku** is a lightweight CLI orchestrator for running and managing multiple local services in development environments.

![screenshot](assets/demo.gif)

## Features

- **Interactive TUI** - Real-time service monitoring with status, CPU, memory, and uptime
- **Service Orchestration** - Tier-based startup ordering
- **Service Control** - Start, stop, and restart services interactively
- **Graceful Shutdown** - SIGTERM with timeout before force kill
- **Profile Support** - Group services for batch operations
- **Readiness Checks** - HTTP, TCP, and log-pattern based health checks
- **Pre-flight Cleanup** - Automatic detection and termination of orphaned processes before starting services
- **Hot-Reload** - Automatic service restart on file changes
- **Log Streaming** - Stream logs from running instances via `fuku logs`

## Installation

### Download from Releases

Download the latest binary from [GitHub Releases](https://github.com/tab/fuku/releases)

**macOS (Apple Silicon):**
```bash
cd ~/Downloads
tar -xzf fuku_v0.14.0_macos_arm64.tar.gz
sudo xattr -rd com.apple.quarantine ~/Downloads/fuku_v0.14.0_macos_arm64/fuku
sudo mv ~/Downloads/fuku_v0.14.0_macos_arm64/fuku /usr/local/bin/fuku
```

**macOS (Intel):**
```bash
cd ~/Downloads
tar -xzf fuku_v0.14.0_macos_amd64.tar.gz
sudo xattr -rd com.apple.quarantine ~/Downloads/fuku_v0.14.0_macos_amd64/fuku
sudo mv ~/Downloads/fuku_v0.14.0_macos_amd64/fuku /usr/local/bin/fuku
```

**Linux:**
```bash
cd ~/Downloads
tar -xzf fuku_v0.14.0_linux_amd64.tar.gz
sudo mv ~/Downloads/fuku_v0.14.0_linux_amd64/fuku /usr/local/bin/fuku
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

Generate a config template with `fuku init`, or create `fuku.yaml` manually in your project root:

```yaml
version: 1

x-readiness-http: &readiness-http
  type: http
  timeout: 30s
  interval: 500ms

x-readiness-tcp: &readiness-tcp
  type: tcp
  timeout: 30s
  interval: 500ms

x-readiness-log: &readiness-log
  type: log
  pattern: "Service ready"
  timeout: 30s

x-logs: &logs
  output: [stdout, stderr]

x-watch: &watch
  include: ["**/*.go"]
  ignore: ["**/*_test.go"]
  shared: ["pkg/common"]
  debounce: 1s

services:
  postgres:
    dir: infrastructure/postgres
    tier: foundation
    readiness:
      <<: *readiness-tcp
      address: localhost:5432

  backend:
    dir: backend
    tier: platform
    readiness:
      <<: *readiness-http
      url: http://localhost:8080/health
    logs:
      <<: *logs
    watch:
      <<: *watch

  web:
    dir: frontend
    tier: edge
    readiness:
      <<: *readiness-http
      url: http://localhost:3000/health

defaults:
  profiles: [default]

profiles:
  default: "*"                    # All services
  backend: [postgres, api]        # Backend services only

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

**HTTP** - Wait for HTTP endpoint to respond with 2xx status:
```yaml
readiness:
  type: http
  url: http://localhost:8080/health
  timeout: 30s
  interval: 1s
```

**TCP** - Wait for TCP port to accept connections:
```yaml
readiness:
  type: tcp
  address: localhost:6379
  timeout: 10s
  interval: 1s
```

**Log** - Wait for pattern in service output:
```yaml
readiness:
  type: log
  pattern: "gRPC server started"
  timeout: 30s
```

### Watch Configuration (Hot-Reload)

Enable automatic service restart on file changes:

```yaml
services:
  api:
    dir: ./api
    watch:
      include: ["**/*.go"]           # Glob patterns to watch
      ignore: ["**/*_test.go"]       # Patterns to ignore
      shared: ["pkg/common"]         # Shared paths (triggers restart)
      debounce: 300ms                # Debounce duration (default: 300ms)
```

### Per-Service Log Output

Control which output streams are logged to the console per service:

```yaml
services:
  api:
    dir: ./api
    logs:
      output: [stdout]              # Only log stdout (default: both)
  worker:
    dir: ./worker
    logs:
      output: [stdout, stderr]      # Log both streams explicitly
```

Valid output values: `stdout`, `stderr`. When omitted, both streams are logged.

### YAML Anchors

Use YAML anchors (`&`) and merge keys (`<<: *`) to avoid repeating common configuration:

```yaml
x-readiness-http: &readiness-http
  type: http
  timeout: 30s
  interval: 500ms

x-watch: &watch
  include: ["**/*.go"]
  ignore: ["**/*_test.go"]
  debounce: 1s

services:
  api:
    dir: ./api
    readiness:
      <<: *readiness-http
      url: http://localhost:8080/health
    watch:
      <<: *watch
```

Top-level keys prefixed with `x-` are ignored by fuku and serve as anchor definitions.

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
