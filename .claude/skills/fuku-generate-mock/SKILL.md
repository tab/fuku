---
name: fuku-generate-mock
description: Generate or regenerate a gomock mock for an interface in the fuku repo. Use when adding a new interface, modifying an existing one, or when tests fail due to stale mocks.
---

# Generate a gomock mock for a fuku interface

Mocks use `go.uber.org/mock` and live alongside the source file:
`foo.go` → `foo_mock.go` in the same package.

## Command

```bash
mockgen \
  -source=internal/path/to/file.go \
  -destination=internal/path/to/file_mock.go \
  -package=packagename
```

Always use **full paths** relative to repo root.

## Rules

- Do NOT add `//go:generate` directives to source files. Run mockgen directly.
- Do NOT modify generated `*_mock.go` files by hand. Re-run mockgen.
- Mock files must be named `*_mock.go` — never `*_mock_test.go`.
- Every consumer-side interface should have a corresponding mock file.
- The mock package name matches the source package name.

## Example

For `internal/app/runner/service.go` defining an interface used by tests:

```bash
mockgen \
  -source=internal/app/runner/service.go \
  -destination=internal/app/runner/service_mock.go \
  -package=runner
```
