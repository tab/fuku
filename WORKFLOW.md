# CI/CD Workflows

## Overview

All workflows live in `.github/workflows/` and form a three-layer protection chain:

```
feature branch → PR → Checks (gate to master)
                        ↓ merge
                      master → Master (post-merge verification + coverage)
                        ↓ tag release
                      release → Release (verify → build → publish)
```

## Workflows

### Checks (`checks.yaml`)

**Trigger:** Pull requests (opened, reopened, synchronize, ready_for_review)

Primary gatekeeper. All jobs must pass before merging to master.

| Job         | Go Versions  | What it does             |
|-------------|--------------|--------------------------|
| Linter      | 1.25         | golangci-lint v2         |
| Vet         | 1.25         | `go vet ./...`           |
| Staticcheck | 1.25         | Static analysis          |
| Tests       | 1.25, 1.26   | `go test -race` (matrix) |
| E2E         | 1.25         | Build binary + e2e tests |

All jobs run in parallel. Concurrency group cancels outdated runs on the same PR.

**Required:** Enable branch protection on `master` with all five checks as required status checks.

### Master (`master.yaml`)

**Trigger:** Push to master (skips docs-only changes), workflow_dispatch

Post-merge safety net. Mirrors the full Checks suite and uploads coverage.

| Job         | Go Versions  | What it does                               |
|-------------|--------------|--------------------------------------------|
| Linter      | 1.25         | golangci-lint v2                           |
| Vet         | 1.25         | `go vet ./...`                             |
| Staticcheck | 1.25         | Static analysis                            |
| Tests       | 1.25, 1.26   | `go test -race` (matrix)                   |
| E2E         | 1.25         | Build binary + e2e tests                   |
| Codecov     | 1.25         | Coverage upload (runs after all jobs pass) |

Ignored paths: `docs/**`, `assets/**`, `**.md`, `LICENSE`, `.github/workflows/pages.yaml`

### Release (`release.yaml`)

**Trigger:** GitHub release (released event)

Final gate before code reaches users via GitHub Releases and Homebrew.

| Job     | Depends on  | What it does                           |
|---------|-------------|----------------------------------------|
| Verify  | —           | Tests with race detector + build + e2e |
| Release | Verify      | GoReleaser (cross-compile + publish)   |
| Sentry  | Release     | Create Sentry release with commits     |

If Verify fails, no artifacts are built or published.

### Pages (`pages.yaml`)

**Trigger:** Push to master (only `docs/**`, `assets/**`, `.github/workflows/pages.yaml`), workflow_dispatch

Builds and deploys the documentation site to GitHub Pages. Independent from Go CI.

## Branch Protection

For the pipeline to be truly bulletproof, configure branch protection on `master`:

- Require status checks to pass: Linter, Vet, Staticcheck, Tests (version: 1.25), Tests (version: 1.26), E2E
- Require branches to be up to date before merging
- Require pull request reviews (optional but recommended)

## Development Flow

1. Create `feature/*` or `fix/*` branch from `master`
2. Push branch — Checks workflow runs on PR
3. Code review + all checks green — merge to `master`
4. Master workflow re-verifies merged code and uploads coverage
5. When ready to release — create a GitHub release with a `v*` tag
6. Release workflow verifies, builds, and publishes
