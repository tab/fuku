# fuku

**fuku** is a lightweight CLI orchestrator for running and managing multiple local services in development environments. It's designed for speed, simplicity, and readability.

## Features

- **Service Orchestration**: Automatic dependency resolution with topological sorting
- **Concurrent Execution**: Start services in proper dependency order with controlled timing
- **Process Management**: Signal handling (SIGINT, SIGTERM) with graceful shutdown
- **Profile Support**: Group services into logical profiles for batch operations
- **Environment Integration**: Automatic `.env.development` file detection and loading
- **Structured Logging**: Clean, prefixed log streaming from all services
- **Makefile Integration**: Services run via `make run` in their directories
- **YAML Configuration**: Simple, readable configuration format

## Installation

### From Source

```bash
git clone <repository-url>
cd fuku
go build -o fuku ./cmd
```

### Binary Installation

```bash
# Copy the binary to your PATH
cp fuku /usr/local/bin/
```

## Usage

### Basic Commands

```bash
# Run services with the default profile
fuku

# Run services with a specific profile
fuku --run=<profile>
fuku run <profile>

# Show help
fuku help

# Show version
fuku version
```

### Examples

```bash
# Run all services
fuku --run=default

# Run core services only
fuku --run=core

# Run minimal services for quick testing
fuku --run=minimal
```

## Configuration

Create a `fuku.yaml` file in your project root:

```yaml
version: 1

services:
  api:
    dir: ./api
    depends_on: [database]

  web:
    dir: ./frontend
    depends_on: [api]

  database:
    dir: ./database
    # No dependencies

profiles:
  default:
    include: [database, api, web]

  core:
    include: [database, api]

  minimal:
    include: [database]

logging:
  format: console    # console or json
  level: info        # debug, info, warn, error
```

### Service Configuration

Each service can be configured with:

- **`dir`**: Directory path (relative or absolute) where the service is located
- **`depends_on`**: Array of service names this service depends on

### Profile Configuration

Profiles define groups of services to run together:

- **`include`**: List of service names to include in this profile
- Use `*` to include all services

### Service Requirements

Each service directory must have:

1. **Makefile** with a `run` target
2. **Optional**: `.env.development` file (automatically loaded as ENV_FILE)

Example service Makefile:
```makefile
run:
	npm start
```

## Architecture

fuku is built with clean architecture principles:

- **CLI Layer**: Command parsing and user interaction
- **Application Layer**: Lifecycle management and coordination
- **Runner Layer**: Service orchestration and process management
- **Config Layer**: Configuration loading and validation

### Key Components

- **Dependency Injection**: Uses Uber FX for clean dependency management
- **Structured Logging**: Built on zerolog for performance and clarity
- **Interface Design**: All major components are interface-based for testability
- **Signal Handling**: Proper cleanup and graceful shutdown
- **Error Handling**: Comprehensive error wrapping and context

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

### Testing

The project includes comprehensive tests:

- **Unit Tests**: All packages have >50% coverage
- **Integration Tests**: CLI and service orchestration scenarios
- **Mock-based Testing**: Using go.uber.org/mock for isolation

### Project Structure

```
cmd/                    # Application entry point
internal/
├── app/               # Application container and lifecycle
├── cli/               # Command-line interface
├── runner/            # Service orchestration engine
└── config/            # Configuration and logging
```

## Use Cases

fuku is perfect for:

- **Microservices Development**: Start all dependent services with one command
- **Full-Stack Development**: Coordinate frontend, backend, and database services
- **Integration Testing**: Spin up complete service environments

## Examples

### Simple Web Application

```yaml
version: 1

services:
  api:
    dir: ./backend
    depends_on: []

  web:
    dir: ./frontend
    depends_on: [api]

profiles:
  default:
    include: [api, web]

  backend-only:
    include: [api]
```

### Microservices Architecture

```yaml
version: 1

services:
  auth-service:
    dir: ./services/auth
    depends_on: [postgres, redis]

  user-service:
    dir: ./services/users
    depends_on: [postgres, auth-service]

  api-gateway:
    dir: ./gateway
    depends_on: [auth-service, user-service]

  postgres:
    dir: ./infrastructure/postgres

  redis:
    dir: ./infrastructure/redis

profiles:
  default:
    include: "*"

  core:
    include: [postgres, redis, auth-service]

  testing:
    include: [postgres, auth-service, user-service]
```

## About the Name

The name fuku (福) means "good fortune" or "blessing" in Japanese. It's also inspired by the legendary Japanese jazz pianist Ryo Fukui, reflecting the tool's focus on orchestration and harmony in development workflows.

## License

Distributed under the MIT License. See `LICENSE` for more information.
