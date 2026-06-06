# fuku — Pull Request Review Policy

## Purpose

This document defines the **PR review process** for the **fuku** repository.

> **Canonical source.** Code rules live in `CLAUDE.md` and `.claude/skills/*/SKILL.md`. This document covers only the review process: severity, PR hygiene, breaking-change handling, and output format. When citing a code-rule violation, reference the CLAUDE.md section heading or the relevant skill.

---

# 1. Severity

Findings are classified as:

- **BLOCKER** — must be fixed before merge
- **MAJOR** — should be fixed before merge
- **MINOR** — improvement suggestion
- **OPTIONAL** — recommendation only

PRs are not approved when BLOCKERS exist.

Assignment:

- CLAUDE.md rules marked **non-negotiable** or that name **BLOCKER** explicitly → **BLOCKER**
- other CLAUDE.md / skill rule violations → **MAJOR**
- undeclared breaking changes (see § 3) → **BLOCKER**
- style notes not covered by CLAUDE.md → **MINOR** or **OPTIONAL**

---

# 2. PR Hygiene

## 2.1 Summary

Every PR needs a **Summary** that explains:

- what the change is for
- the meaningful pieces of the change (CLI behavior, config schema, lifecycle, bus messages, concurrency, error handling, observability, TUI, tests)

If the diff goes beyond the stated intent, request clarification — do not guess.

## 2.2 PR Title (MAJOR)

Conventional commit format: `type(scope): description`.

Types: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `perf`. Type is lowercase. Scope is optional. Description is concise.

## 2.3 Commits (MAJOR)

- conventional commit format on each commit
- atomic — one logical change per commit
- no generic messages (`fix`, `update`, `wip`)

---

# 3. Breaking Changes (BLOCKER if undeclared)

A change is breaking if it:

- removes or renames CLI commands or flags
- changes CLI output that scripts may depend on
- modifies `fuku.yaml` schema incompatibly
- changes signal handling, startup/shutdown ordering, or bus message structures
- changes public interface method signatures

If a breaking change is present but not declared in the PR summary → **BLOCKER**. If uncertain, assume breaking and flag.

---

# 4. Output Format

```
#### PR METADATA
- (Rule 2.2) PR title: …
- (Rule 2.3) Commit quality: …

#### BLOCKERS
- [file:line] (CLAUDE.md > <section> | skill `<name>` | Rule 2.x / 3) Description
  → Fix

#### MAJOR
- [file:line] (CLAUDE.md > <section> | skill `<name>`) Description
  → Fix

#### MINOR
- [file:line] Description
  → Suggestion

#### OPTIONAL
- [file:line] Description
  → Suggestion

#### SUMMARY / INTENT MISMATCH
- … or `None`

#### BREAKING CHANGE
- Yes / No (+ impact if Yes)

#### VERDICT
APPROVE / REQUEST CHANGES / NEEDS DISCUSSION
```

Rules:

- code findings need `file:line`
- BLOCKER and MAJOR findings cite the rule — a CLAUDE.md section heading, a skill name, or a rule number from this document
- group findings by file
- include a concrete fix for BLOCKER and MAJOR
- if a section is empty, write `None`
- prefer CLAUDE.md section headings over restating the rule
