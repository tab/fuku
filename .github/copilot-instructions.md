---
description: 'Instructions for reviewing and generating Go code in the fuku repository'
applyTo: '**/*'
---

# Copilot Instructions

For Go code generation and PR review in this repository, the canonical sources are:

- **`CLAUDE.md`** — the code rules. **Authoritative.**
- **`ARCHITECTURE.md`** — the layers, the features and their wiring.
- **`.claude/skills/*/SKILL.md`** — procedures: `verify`, `add-test`, `generate-mock`, `config`.
- **`.github/CODE_REVIEW.md`** — PR review process: severity model, PR hygiene, breaking-change detection, output format.
- **`.github/CODE_REVIEW_PROMPT.md`** — the multi-pass review checklist.

Do not rely on generic best practices or external style guides. When `CLAUDE.md` and another document disagree, `CLAUDE.md` wins.
Report the drift.

This applies to every LLM assistant in this repo: Copilot, Codex, Claude Code, Cursor.

---

## Project Context

**fuku** is a lightweight CLI orchestrator for local services in development, written in Go.

Key technologies:

- **Dependency injection**: Uber FX
- **CLI framework**: Cobra
- **Configuration**: Viper (YAML format, `fuku.yaml`)
- **Logging**: `log/slog` over a zerolog handler
- **Error tracking**: Sentry
- **Testing**: testify (assertions) + go.uber.org/mock (mock generation)
- **TUI**: Bubble Tea / Bubbles / Lipgloss
- **Linting**: golangci-lint v2 with strict configuration (`.golangci.yaml`)
- **CI**: GitHub Actions (linter, tests with race detector, e2e)

---

## Fallback Behavior

When `CLAUDE.md` and the skills say nothing, decide in this order:

1. **Existing repository patterns.** How the codebase already solves it.
2. **Idiomatic Go.** Naming, error handling, interfaces, concurrency primitives.
3. **The [Uber Go Style Guide](https://github.com/uber-go/guide).** Guidance only. It never overrides `CLAUDE.md`.

A suggestion from 2 or 3 is reported as **OPTIONAL**, never as a violation.
