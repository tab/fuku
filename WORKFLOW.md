# CI/CD Workflows

## Overview

The workflows live in `.github/workflows/`. Three gates and one deployment:

```
feature branch → PR → Checks + Conventions (gate to master)
                        ↓ merge
                      master → Master (post-merge verification + coverage)
                        ↓ tag release
                      release → Release (verify → build → publish)

master, docs/** or spec/openapi.yaml changed → Pages (docs site)
```

## Workflows

### Checks (`checks.yaml`)

**Trigger:** Pull requests (opened, reopened, synchronize, ready_for_review)

The gate to master. Every job must pass.

| Job         | What it does                                                                        |
| ----------- | ----------------------------------------------------------------------------------- |
| Linter      | golangci-lint v2, which runs `govet` and `staticcheck`                              |
| Tests       | `make test:race` on each Go version of the matrix                                   |
| E2E         | `make build && make test:e2e`                                                       |
| Plugin      | ktlint and `buildPlugin` for the JetBrains plugin                                   |
| Codecov     | Coverage upload. Runs after Linter, Tests and E2E pass                              |
| Spec        | `openapi.sh` (the spec ships with an API change) and `links.sh` (doc links resolve) |

The jobs run in parallel. A new push cancels the older run of the same PR.
The `contract-unchanged` label waives the spec check for a PR that touches the API package without changing the contract.

### Conventions (`conventions.yaml`)

**Trigger:** Pull requests (opened, reopened, edited, synchronize, ready_for_review). Skipped for dependabot

One job, `Conventions`. The title is a scoped Conventional Commit (`subject.sh`).
Every commit is scoped and carries no AI attribution (`commits.sh`).
It is a separate workflow so a title edit reruns it without cancelling the code jobs.

### Master (`master.yaml`)

**Trigger:** Push to master (skips docs-only changes), workflow_dispatch

Re-runs the Go jobs after the merge and uploads coverage.

| Job         | What it does                                           |
| ----------- | ------------------------------------------------------ |
| Linter      | golangci-lint v2, which runs `govet` and `staticcheck` |
| Tests       | `make test:race` on each Go version of the matrix      |
| E2E         | `make build && make test:e2e`                          |
| Codecov     | Coverage upload. Runs after Linter, Tests and E2E pass |

Ignored paths: `docs/**`, `assets/**`, `**.md`, `LICENSE`, `.github/workflows/pages.yaml`

### Release (`release.yaml`)

**Trigger:** GitHub release (released event)

The gate before a release reaches GitHub Releases and Homebrew.

| Job              | Depends on | What it does                                             |
| ---------------- | ---------- | -------------------------------------------------------- |
| Verify           | —          | `make test:race`, `make build`, `make test:e2e`          |
| Release          | Verify     | GoReleaser (cross-compile + publish)                     |
| JetBrains Plugin | Verify     | `buildPlugin` and upload of the zip to the release       |
| Sentry Release   | Release    | Create the Sentry release with commits                   |

If Verify fails, nothing is built or published.

### Pages (`pages.yaml`)

**Trigger:** Push to master (only `docs/**`, `spec/openapi.yaml`, `assets/**`, `.github/workflows/pages.yaml`), workflow_dispatch

Copies `spec/openapi.yaml` into `docs/public/`, builds the Astro site and deploys it. Independent of the Go CI.

## Branch Protection

Configure branch protection on `master`:

- required checks: Linter, both Tests matrix jobs, E2E, Plugin, Spec, Conventions
- the branch must be up to date before a merge
- pull request reviews are recommended

## Development Flow

1. Branch `feature/*` or `fix/*` from `master`
2. Open a PR. Checks and Conventions run
3. Review, green checks, merge
4. Master re-verifies and uploads coverage
5. Create a GitHub release with a `v*` tag
6. Release verifies, builds and publishes
