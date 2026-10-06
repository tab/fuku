# plugins/agents/e2e

Manual end-to-end tests of the `fuku` skill. A real Claude Code and a real Codex drive a real fuku through the skill.

## Why it exists

The [structure check](../tests/test_plugin.py) proves the skill is well formed. It cannot prove that an agent follows it.
This sweep sends common tasks to both hosts and checks what happened.

What a run judges:

- fuku's end state: which fuku runs, which services run, which PIDs changed
- the commands in the transcript: what the skill requires and what it forbids
- the cost: tool calls, input tokens, seconds and USD against the scenario budget

What it never judges:

- the agent's prose. A correct answer with forbidden commands fails. A clumsy answer with the right commands passes
- the quality of a diagnosis. S3 proves the agent fetched the evidence, not that it explained it well

The boundary: this folder holds the runner, the fixture and the scenarios. It ships inside the plugin, outside `skills/`, so no host loads it as a skill.

## How it works

1. `make build` builds fuku. The runner puts that binary first on `PATH`
2. For each host, scenario and repeat, the runner copies `fixture/` into a fresh temp project, with a free API port and a random token
3. It runs the scenario's setup, such as starting fuku before the agent arrives
4. It starts the host headless with the scenario's prompt and a hard time limit of twice the time budget
5. It reads the host's JSON events into one list of commands, outputs and totals
6. It checks fuku's end state, then tears the project down, then checks the commands and the budgets
7. After the last run it writes the report and compares the real host config with its state before the sweep

Isolation:

- nothing is installed into the user's hosts
- Claude Code loads the plugin from the working tree with `--plugin-dir` and reads only project settings
- Codex gets a template `CODEX_HOME` per sweep, with the plugin installed from the working tree. Each run copies it into a fresh private home
- the installed skill must equal the tree's, byte for byte and with its executable bits
- each host gets an allowlisted environment: `PATH`, `HOME`, `USER`, `LOGNAME`, `SHELL`, `TMPDIR`, `LANG`, `LC_*`, `TERM`. No `ANTHROPIC_*` or `OPENAI_*` variable is passed
- a proxy or CA variable can be passed with `E2E_PASS_ENV=NAME,NAME`. Its values of 8 or more characters are then redacted from everything the sweep keeps

Credentials:

- the Codex login is copied into each run's private home with mode 600 and deleted after the run, also on a timeout, Ctrl-C or SIGTERM
- a run that reads a host login fails
- a call that names a login source, such as `auth.json` or the keychain lookup, has its whole output replaced with `<redacted>`
- login text found only by its content is replaced where it appears
- when Codex refreshes its login during a run, the sweep prints a warning. The real `~/.codex/auth.json` is never written, so you may need `codex login`
- the API token of each run is redacted. A run fails when it appears anywhere in the host's raw events, also in a message before the final answer
- a host error keeps at most 300 characters of stderr. The secrets are redacted first, then the text is cut

## How to use it

Prerequisites:

- Claude Code and Codex are installed and logged in through their CLIs
- fuku is built: `make build`

Commands:

```sh
make build
make test:agents-e2e
make test:agents-e2e HOST=codex SCENARIO=S2 REPEATS=1
```

- `HOST` is `claude` or `codex`. Unset runs both
- `SCENARIO` is a scenario ID, or a comma-separated list. Unset runs all
- `REPEATS` is the number of runs per host and scenario. The default is 3
- `make` also reads `HOST` from the environment. Some shells export it as the machine name, and the target then exits 2 with "HOST must be one of claude, codex"
- in that case pass it explicitly: `make test:agents-e2e HOST=` runs both hosts

Cost and quota:

- every run is a real agent session on your own subscription
- a full sweep is 2 hosts × 7 scenarios × 3 repeats, so 42 runs
- a Claude Code run cost about $0.25 to $0.40 in the first full sweep. The Claude Code half of that sweep cost about $6.50
- Codex runs draw on your Codex plan's quota
- the scenario budgets cap one run at $0.75 to $0.80
- each budget is about twice the worst run of the first 42-run sweep
- an account limit marks the run as a host outage, stops that host and makes the sweep exit 1
- a missing login stops that host the same way
- SIGTERM stops a sweep the way Ctrl-C does: the host group is killed, the auth copy and the temp project are removed, and the runner exits 143.
  A repeated SIGTERM is ignored, so it cannot cut the cleanup short.
  The status lines of the finished runs stay on stdout. No report file, table or guard check follows

Do not use Claude Code or Codex for other work while a sweep runs.
The guards compare the state before and after the sweep:

- `~/.codex/config.toml` and `~/.claude/settings.json` by content hash
- the paths under `~/.codex/plugins` whose name contains `fuku`
- every file, link and directory under `~/.claude/plugins` by relative path and content hash.
  A directory the guard cannot read is recorded as unreadable. Nothing is capped, so a large plugin tree makes the guard slower

A plugin install, a changed metadata value or an edited cached file from another session fails the sweep.
The `~/.claude/plugins` guard compares file contents, so a Claude Code session or a marketplace update that changes that tree during a sweep fails the guard.
Do not run Claude Code plugin commands while a sweep runs.

The offline self-checks run in `make test:agents-plugin` and spend no tokens.
The tests that need a built fuku skip there. Run them with `make build && FUKU_BIN=$PWD/cmd/fuku python3 -m unittest discover -s plugins/agents/tests -p 'test_e2e_*.py'`.

## The report

The sweep writes to a new temp folder outside the repository and prints its path.

- `report.json` holds, per host and scenario, the pass rate and each failed check with its reason
- it also holds the medians of tool calls, input tokens, output tokens, seconds and USD, the host version and the model
- each run has flags: `over_budget`, `timed_out`, `host_outage`, `credentials_read` and `login_rotated`
- the sweep lists the largest fuku command outputs by bytes
- one file per run, `<host>-<scenario>-<n>.json`, holds the redacted commands and outputs. Raw host output is never written
- it also holds `answer`, the agent's final message, redacted and cut to 4000 characters. It is empty when the host gave none

Nothing in the report should be a secret. It still holds absolute paths and command output, so do not share it blindly.

Exit codes:

- `0`: every scenario passed every repeat on every host, and no guard failed
- `1`: a scenario fell below 100%, was not run, a host had an outage or no login, or a guard failed
- `2`: a usage error, a scenario file error or a missing fuku binary. No host starts

## Scenarios

One JSON file per scenario in `scenarios/`, named by its ID. The keys are exactly `prompt`, `profile`, `setup`, `checks` and `budget`.

- `profile` must be a profile of `fixture/fuku.yaml`
- `setup` runs in order before the prompt:
  - `start_fuku`
  - `override:<template>` with a template from `fixture/overrides/`
  - `wait_service_exited:<service>`
  - `edit_service_command:<service>`
- `checks` names state checks and command checks:
  - state: `fuku_running`, `services_running`, `same_fuku_pid`, `same_service_pids`, `fuku_pid_changed`, `served_by_built_fuku`, `service_pid_changed:<service>`, `other_service_pids_same:<service>`, `service_command_is:<service>`, `service_stopped:<service>`
  - commands: `run_detached_once`, `status_read`, `log_read_with_marker`, `service_log_read:<service>`, `no_fuku_run`, `no_default_run`, `no_fuku_stop`, `no_kill`, `no_api_restart`
- `budget` holds `tool_calls`, `input_tokens`, `seconds` and `usd`. `usd` applies only where the host reports it

Every run also checks, whatever the scenario says:

- no foreground `fuku run` and no `fuku logs` without `--no-follow`
- no whole-file read of a fuku config. A bounded single-key read such as `grep -n '^ *listen:' fuku.yaml` is allowed
- no kill of fuku or a service, and no removal of a fuku socket or lock.
  A `pkill` or `killall` pattern counts as a kill when it matches `fuku`, a service program such as `sleep 3601`, or a service dir. So `pkill sleep` fails
- no `--config` outside the temp project
- the API token never appears, the skill was loaded and no host login was read.
  Claude Code loads the skill when its `Skill` call succeeds from the working-tree plugin.
  Codex loads it when one successful call runs a reader such as `cat`, `sed` or `nl` on the installed `SKILL.md` and its output holds every line of the file. Listing or echoing the path, or a partial read, does not count

An unknown key, setup name or check name is refused before any host starts.

## Changing it

- the adapters were written for Claude Code 2.1.289 and codex-cli 0.159.3. An event the adapter cannot read fails the run as `unreadable`.
  When a host changes its event shape, record a new sanitized sample in `../tests/samples/` and update the parser
- the command checks read shell text, not intent. They miss a command hidden in a script file, in `python -c` or in a command substitution such as `$(fuku run)`
- command checks compare sets and counts, never order. Runs differ in order
- keep every secret out of what the sweep writes: redact first, then cut
- keep this folder free of a `SKILL.md`. The structure check expects exactly one skill in the plugin
