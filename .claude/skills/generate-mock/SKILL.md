---
name: generate-mock
description: Generate or regenerate a gomock mock for a Go interface. Use when adding a new interface, modifying an existing one, or when tests fail due to stale mocks.
---

# Generate a gomock mock for an interface

Mocks use `go.uber.org/mock` and live in the package whose tests use them, next to those tests: the mocks `check_test.go` needs go into `check_mock_test.go` in the same package.
Interfaces are consumer-owned (`updater.ReleaseSource`, `services.Launcher`), so the mock always sits in the package that declares the interface, never in the package that implements it.
The two shared bus interfaces (`contracts.Publisher`, `contracts.Subscriber`) are the exception: a package that asserts on them generates its own `contracts_mock_test.go` from `fuku/internal/contracts`, because mockgen reads one source package per file.

## New mock

Package mode, naming only the interfaces a test in that package mocks:

```bash
mockgen \
  -destination=internal/app/updater/check_mock_test.go \
  -package=updater \
  fuku/internal/app/updater ReleaseSource

mockgen \
  -destination=internal/app/updater/contracts_mock_test.go \
  -package=updater \
  fuku/internal/contracts Publisher
```

Package mode compiles the package, so the interface must live in a non-test file and the package must build.

## Regenerate an existing mock

The generating command is on line 6 of every generated file; run it again unchanged:

```bash
sed -n '6p' internal/app/updater/check_mock_test.go   # mockgen -destination=... -package=updater fuku/internal/app/updater ReleaseSource
```

To add an interface to an existing file, or drop one no test uses any more, edit the interface list of that command and run it.

## Rules

- Do NOT add `//go:generate` directives to source files. Run mockgen directly.
- Do NOT modify generated mock files by hand. Re-run mockgen.
- Always use **full paths** relative to the repo root.
- A mock is always a `*_mock_test.go`; never create a `*_mock.go` and never import a mock from another package (`find . -name '*_mock.go'` stays empty).
- Generate a mock only for an interface a test asserts on (a call count or its arguments); a dependency no test asserts on takes a no-op stand-in instead (see `add-test`, "Mocks: one layer down").
  A `Logger` mock exists only where a test asserts a log line.
- The mock package name matches the source package name.
- Never use `-source` mode: it mocks every interface in the file, used or not.

## Example

`internal/adapters/process/preflight.go` declares `Pool`, and `preflight_test.go` asserts the slots `Cleanup` acquires:

```bash
mockgen \
  -destination=internal/adapters/process/preflight_mock_test.go \
  -package=process \
  fuku/internal/adapters/process Pool
```
