# MCP server plan

Feature: [feature.md](feature.md)

Status: implemented

Phase: pr review

Current step: PR

## Done when

- Every acceptance criterion in `feature.md` holds
- `make check`, `make test:race`, `make test:e2e`, `make test:agents-plugin` with `FUKU_BIN`, `make docs`,
  `npm run build` in `docs/` and the depguard audit pass
- One sweep run of every scenario passes on both hosts

## Steps

- [x] 1. Core: the REST service actions settle with an opt-in `wait`; a stopped service settles with `watching: false` (AC6, AC10)
- [x] 2. The log socket `status` frame carries the run mode and the API address (AC11)
- [x] 3. `fuku mcp`: the command, its composition, the stdio server, the stderr banner and call lines (AC1, AC12)
- [x] 4. Read tools `status`, `config`, `logs` and `doctor`, with the config loaded fresh on each call (AC2–AC4, AC8, AC9)
- [x] 5. Write tools `run`, `stop` and `service` (AC5–AC7)
- [x] 6. Doctor: a port held under this project's own fuku is a note; a load failure names the override too
- [x] 7. The agent's rules (AC13)
  - Instructions: the first 512 characters stand alone, the tool map follows
  - Tool descriptions: each control tool is the only way to do its job
  - Skills: `fuku` for every task, `fuku-config` for config edits; shown as `fuku` and `fuku config` in Codex
- [x] 8. The agent plugin for Claude Code and Codex: the MCP config, the skills and the Codex tool approval (AC13)
- [x] 9. The sweep: 15 scenarios in five groups and a limit, graded on tool calls (AC13)
  - The grader fails CLI probes, scripts and config file reads, and keeps host errors apart from agent errors
- [x] 10. Docs: `README.md`, `ARCHITECTURE.md`, the `adapters/mcp` README, the e2e README and the site
- [x] 11. The token stays out of every tool result, error and call line, and the grader reads config access precisely (CX-1 to CX-10, AC9, AC13)
- [x] 12. `mcp` uses `contracts.Action` instead of its own action type
- [x] 13. Renames in `mcp`: the REST interface is `Client` in `client.go`, the log-socket interface is `Instance`, and plain function fields replace the dialer types
- [x] 14. One comment form for const groups in `mcp`
- [x] 15. The server instructions are one raw string literal, one rule per line, as `cli/help.go` does (AC13)
- [x] 16. One config lookup: the config adapter loads a project's config from a given directory and `mcp` drops its copies, so `CLAUDE_PROJECT_DIR` and the working directory cannot point at two projects (AC2, AC8)
- [x] 17. A small bubbletea view in `tui` renders the `fuku mcp` banner and call lines on a terminal, with plain lines on a pipe, no input and stderr only (AC12)
- [x] 18. The MCP composition resolves the theme from stderr or documents the fixed one

## Gates

- [x] Plan review – PASS (round 2, codex, gpt-6.1-sol, high)
- [x] Code review – round 2 PASS WITH FOLLOW-UPS (risk review: general claude/opus/high + agent control boundaries codex/gpt-6-astra/high); G2 accepted risk, G3 deferred
