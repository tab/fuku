# Agent skill end-to-end tests

## Goal

`make build && make test:agents-e2e` runs the `fuku` skill in Claude Code and in Codex on common tasks and says whether it works.
It judges fuku's end state, the commands in the transcript and the cost. It never judges the model's prose.

## Why

The [agent skill](../20261004-agent-skill/feature.md) has a structure check, but nothing proves that an agent follows it. A spike ran one task on each host. It found what no static check sees:

- `fuku logs --no-follow` put about 14.5 KB into the context at debug level. 56% of it was escape codes
- Every agent ran `fuku logs` to learn which services run, though the `run -d` summary says it. Codex read the 6.3 KB API reference with the API off

We own both sides: fuku and the skill. A cheap, repeatable sweep shows when a change to one breaks the other.

## Scope

In:

- A Python standard-library runner in `plugins/agents/e2e/`, with one adapter per host, `make test:agents-e2e` and a short `README.md`
- One data file per scenario. Each has an ID, so it can be cut. "Same fuku" means the fuku PID and every service PID are unchanged:
  - **S1** – start the default profile. Setup: nothing runs. Prompt: `start services in fuku`. Must: one `fuku run -d` of the default profile, named or not, that exits 0.
    End: fuku and its services run
  - **S2** – attach. Setup: fuku runs. Prompt: asks which services run. Must: one successful doctor or bounded log read. Never: `fuku run`, `fuku stop`. End: same fuku
  - **S3** – find a failure. Setup: one service exits after it is ready and prints a known marker. Must: a bounded log read whose output holds the marker. End: same fuku.
    This proves the agent fetched the evidence. It does not grade the explanation
  - **S4** – restart one service and read its logs. Setup: fuku runs with the API. Prompt: `restart service api and check the logs`.
    Must: that service's PID changes, and one successful `fuku logs api` with `--no-follow`, `--no-ui` and a `--tail`. End: same fuku PID and other service PIDs.
    The token never appears in the transcript
  - **S5** – apply a config change. Setup: fuku runs, then the runner edits one service command. Must: a new fuku PID and the new command running. Never: an API restart call
  - **S6** – reuse a running fuku. Setup: fuku runs. Prompt: `start services in fuku`. Never: `fuku stop`, a kill. End: same fuku
  - **S7** – start a named profile. Setup: nothing runs. Prompt: `run core profile with fuku`. The fixture's `core` profile holds `api`, not `worker`.
    Must: one `fuku run core -d` that exits 0. Never: a `fuku run` of the default profile, named or not. End: fuku and `api` run, `worker` does not
- Budgets per scenario, and a sweep report with pass rates, medians and the largest fuku outputs by bytes
- The skill text changes K1, K2 and K3 the spike justifies, in AC19 to AC21. Each one can be cut

Out:

- Any change under `internal/`, `cmd/` or `e2e/`
- The fuku fixes the spike found: [BL-019 to BL-024](../backlog.md)
- A CI job, a pre-push line, `claude plugin eval` suites and other hosts

## How it works

1. `make build` builds fuku, as for `make test:e2e`. `make test:agents-e2e` puts that binary first on `PATH` and fails when it is missing
2. For each host, scenario and repeat, the runner copies the fixture into a fresh temp project with a free API port and a random token
3. It runs the scenario's setup, such as starting fuku or making the config edit the user "already made"
4. It starts the host headless, with the plugin from the working tree, the scenario's prompt and none of the user's own config
5. It reads the host's JSON events into one list of commands with their outputs and totals. It checks fuku's end state, the commands and the budgets
6. It stops fuku, kills what is left and removes the new lock file. After the last run, it writes the sweep report

## Acceptance criteria

- **AC1** – a scenario file with an unknown key or an unknown check name is rejected before any host runs
- **AC2** – a Claude Code run loads the skill from the working tree
- **AC3** – a Codex run uses an installed skill identical to the working-tree skill
- **AC4** – a run fails when a fuku other than the built one serves the project
- **AC5** – a run fails when a `Must`, `Never` or `End` of its scenario does not hold
- **AC6** – a run fails on a forbidden command: a foreground `fuku run`, `fuku logs` without `--no-follow`, a whole-file read of a fuku config, a kill of fuku or a service, or a socket removal
- **AC7** – a run fails when the API token appears anywhere in its transcript
- **AC8** – a run fails when the host did not load the skill
- **AC9** – a run over any budget is reported as over budget and fails
- **AC10** – a run past the hard time limit is killed and reported as timed out
- **AC11** – after teardown, no fuku, service process, socket or new lock file of the run remains
- **AC12** – after a sweep, the real `~/.codex/config.toml` and the set of fuku paths under `~/.codex/plugins` equal their state before it
- **AC13** – after a sweep, the real `~/.claude/settings.json` and `~/.claude/plugins` are unchanged
- **AC14** – the copied Codex `auth.json` is deleted after each run, also after a timeout or an interrupt
- **AC15** – a sweep exits non-zero when a scenario's pass rate is below the threshold
- **AC16** – the report gives each host and scenario its pass rate, failed checks, medians, host version and model
- **AC17** – the report lists the largest fuku command outputs by bytes
- **AC18** – no `SKILL.md` exists under `plugins/agents/e2e/`, and the structure check still finds exactly one skill
- **AC19** – K1: the skill names the `run -d` summary as the answer to which services run
- **AC20** – K2: the skill reads the API reference only after it finds `server.listen` set
- **AC21** – K3: every `fuku logs` command in the skill carries `--no-ui`, which drops the banner

## Assumptions

- Results vary between runs, host versions and models. The signal is a pass rate over repeats, valid for the recorded versions and models
- The adapters fit Claude Code 2.1.289 and codex-cli 0.159.3. An event the adapter cannot read fails the run
- Codex can install the plugin into an empty `CODEX_HOME` without a prompt. The spike shows that end state, but not the commands that reach it
- `--setting-sources project` and a private `CODEX_HOME` keep the user's plugins, hooks and config out. Claude Code may touch `~/.claude.json` on start, so AC13 leaves it out
- fuku runs every service command through `sh -c`, so the fixture's services are shell lines. The suite builds no stub and imports nothing from `e2e/`. A service that exits after it is ready leaves fuku running, so S3 can read its logs. A failed start stops every service, so S3 cannot use one

## Contracts

- A scenario is one JSON file named by its ID, with the keys `prompt`, `profile`, `setup`, `checks` and `budget`
- `setup` and `checks` take names from closed lists in the runner, which the README documents. Every run also checks AC6, AC7 and AC8. Command checks compare sets and counts, never sequences, because runs differ in order
- `budget` holds `tool_calls`, `input_tokens` (cached included), `seconds` and `usd`. `usd` applies only where the host reports it
- `HOST=claude|codex`, `SCENARIO=<id>` and `REPEATS=<n>` narrow a sweep. The default is both hosts, every scenario and 3 repeats
- A sweep writes `report.json` and the transcripts to a temp folder outside the repository and prints its path. It exits 0 when every scenario meets the threshold, 1 when one does not, and 2 for a usage or scenario-file error

## Decisions

- The suite lives in `plugins/agents/e2e/`, outside `skills/`, so no host loads it as a skill. An install still copies it into the plugin cache, as it copies `tests/`
- One standard-library Python runner for both hosts, not `claude plugin eval`, because it covers Codex too
- No install into the user's real hosts and no restore: Claude Code reads the working tree through `--plugin-dir`, and Codex gets a private `CODEX_HOME`.
  An interrupted sweep cannot leave a test skill installed, and no other session sees it
- Three repeats, and all must pass. A budget overrun fails the run. Each host runs its default model, which the report records
- The runner's offline self-checks run in `make test:agents-plugin`, so CI catches a broken parser without spending tokens
- Manual only: a Claude Code run cost about $0.45 in the spike, a sweep of 3 repeats is 42 runs, and both hosts must be logged in
- The runner makes the S5 config edit before the prompt. Claude Code's edit tool needs a whole-file read, which the skill forbids
- Codex runs without a login shell and with sandbox network access. The login shell found the released fuku first, and the sandbox blocks the socket without network
