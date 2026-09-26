---
name: verify
description: Run the full fuku verification loop (format, lint, vet, test, race, e2e, docs) before committing. Use when asked to verify changes, run lint, run tests, check that changes pass CI, or before any commit/push.
---

# fuku verification loop

Run the steps in order. Fix each step before the next.

```bash
# 1. Format
make fmt

# 2. Lint — fix any issues, re-run until clean
make lint

# 3. Vet — fix any issues, re-run until clean
make vet

# 4. Tests — fix any failures, re-run until clean
make test

# 5. Race detector — fix any races, re-run until clean
make test:race

# 6. E2E tests — fix any failures, re-run until clean
make build && make test:e2e

# 7. Doc links — fix any broken link, re-run until clean
make docs
```

**Never commit without running every step.**

`make lint` expects golangci-lint v2.13.2. CI pins that version in `checks.yaml`.

## Audits

`make lint` runs `depguard`. It enforces the dependency rules in `CLAUDE.md`. The other audits are one command each.
`deadcode` is installed once with `go install golang.org/x/tools/cmd/deadcode@latest`.
Each must return nothing:

```bash
find . -name '*_mock.go' -not -path './vendor/*'                       # every mock is a *_mock_test.go
find . -name 'module_test.go' -not -path './vendor/*'                  # module.go holds wiring only
grep -rl '"fuku/internal/adapters/config"' --include='*.go' internal/ | grep -v 'internal/adapters/config/\|internal/bootstrap/'
grep -rn 'lipgloss\.NewStyle()' --include='*.go' internal/ cmd/ | grep -v 'terminal/theme.go\|terminal/styles.go'
deadcode ./cmd/... | grep 'internal/'                                  # an export only tests reach is dead code
grep -rn 'time.Sleep' --include='*_test.go' internal                   # a test waits on a channel, never a clock
go run ./.github/scripts/comments                                      # a comment is one line; a body comment names its case
```

A `*_mock_test.go` whose mocks no test uses is stale. Regenerate it with the interfaces still in use. See `generate-mock`.

## Other useful make targets

```bash
make build       # build binary
make coverage    # coverage report
```
