---
description: 'Instructions for reviewing and generating Go code in the fuku repository'
applyTo: '**/*'
---

# Copilot Instructions

For Go code generation and PR review in this repository, the canonical sources are:

- **`CLAUDE.md`** — code-quality rules (architecture, error handling, naming, testing, concurrency, security, logging, etc.). **Treat as authoritative.**
- **`.claude/skills/*/SKILL.md`** — procedure references (`verify` for the verification loop, `add-test` for test patterns, `generate-mock` for mock conventions, `config` for `fuku.yaml`).
- **`.github/CODE_REVIEW.md`** — PR review process: severity model, PR hygiene, breaking-change detection, output format.
- **`.github/CODE_REVIEW_PROMPT.md`** — the multi-pass review checklist.

Do not rely on generic best practices, external style guides, or personal assumptions. When `CLAUDE.md` and any other document drift, **treat `CLAUDE.md` as authoritative** and report the drift.

The same authority applies to any LLM-based assistant working in this repo — GitHub Copilot, OpenAI Codex, Claude Code, Cursor, etc.

---

## Project Context

This repository is **fuku** — a lightweight CLI orchestrator for running and managing multiple local services in development environments, written in Go.

Key technologies:

- **Dependency injection**: Uber FX
- **CLI framework**: Cobra
- **Configuration**: Viper (YAML format, `fuku.yaml`)
- **Logging**: zerolog (structured logging)
- **Error tracking**: Sentry
- **Testing**: testify (assertions) + go.uber.org/mock (mock generation)
- **TUI**: Bubble Tea / Bubbles / Lipgloss
- **FSM**: looplab/fsm
- **Linting**: golangci-lint v2 with strict configuration (`.golangci.yaml`)
- **CI**: GitHub Actions (linter, vet, staticcheck, tests with race detector, e2e)

---

## Fallback Behavior

If a situation is **not explicitly defined in `CLAUDE.md` or any skill**, use the following decision order:

1. **Existing repository patterns**
   Prefer how similar problems are already solved in the current codebase.

2. **Core Go language conventions**
   If no repository pattern exists, fall back to idiomatic Go practices:
   - Go naming conventions
   - standard error handling patterns
   - common interface patterns
   - common concurrency primitives

3. **Uber Go Style Guide (guidance only)**
   The [Uber Go Style Guide](https://github.com/uber-go/guide) may be used as additional guidance when no repository pattern exists.
   Rules from the Uber guide must **not override `CLAUDE.md`** and must be reported only as suggestions.

4. **Optional recommendations**
   Suggestions based on these conventions must be reported under **OPTIONAL / RECOMMENDATIONS**, not as rule violations.
