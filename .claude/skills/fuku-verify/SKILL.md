---
name: fuku-verify
description: Run the full fuku verification loop (format, lint, vet, test, race, e2e) before committing. Use when asked to verify changes, run lint, run tests, check that changes pass CI, or before any commit/push.
---

# fuku verification loop

Run the steps in order. Fix issues at each step before proceeding to the next.

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
```

**Never commit without running every step.**

## Other useful make targets

```bash
make build       # build binary
make coverage    # coverage report
```
