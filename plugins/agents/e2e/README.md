# plugins/agents/e2e

Manual end-to-end tests of the `fuku` and `fuku-config` skills. A real Claude Code and a real Codex drive a real fuku through the plugin's MCP tools.

## Why it exists

The [structure check](../tests/test_plugin.py) proves the skills are well formed. It cannot prove that an agent follows them.
This sweep sends common tasks to both hosts and checks what happened.

What a run judges:

- fuku's end state: which fuku runs, which services run, which PIDs changed
- the tool calls in the transcript: the fuku MCP calls the task needs and the shell commands it forbids
- the cost: tool calls, input tokens, seconds and USD against the scenario budget

What it never judges:

- the agent's prose. A correct answer with forbidden commands fails. A clumsy answer with the right commands passes
- the quality of a diagnosis. S5 proves the agent fetched the evidence, not that it explained it well

This folder holds the runner, the fixture and the scenarios. It ships inside the plugin, outside `skills/`, so no host loads it as a skill.

## How it works

1. `make build` builds fuku. The runner puts that binary first on `PATH`
2. For each host, scenario and repeat, the runner copies `fixture/` into a fresh temp project, with a free API port and a random token
3. It runs the scenario's setup, such as starting fuku before the agent arrives
4. It starts the host headless with the scenario's prompt and a hard time limit of twice the time budget
5. It reads the host's JSON events into one list of calls, outputs and totals. A fuku MCP call keeps its tool name and arguments
6. It checks fuku's end state, then tears the project down, then checks the commands and the budgets
7. After the last run it writes the report and compares the real host config with its state before the sweep

Isolation:

- nothing is installed into the user's hosts
- Claude Code loads the plugin from the working tree with `--plugin-dir` and reads only project settings.
  It starts the plugin's MCP server, allows its tools next to `Bash`, `Read` and `Skill`, and loads no claude.ai connectors.
  A run whose init event names an MCP server other than `plugin:fuku:fuku`, such as a user-scope server, fails as `isolation`.
  A run whose init event shows `plugin:fuku:fuku` as `failed` or `needs-auth` fails as `mcp`, a host problem, not an agent one.
  A stale `plugin:fuku:fuku` entry in `~/.claude/mcp-needs-auth-cache.json` causes it; remove that entry
- Codex gets a template `CODEX_HOME` per sweep, with the plugin installed from the working tree. Each run copies it into a fresh private home
- `codex exec` never asks for approval, so it refuses a tool that needs one.
  The runner approves the fuku tools for each run with `-c plugins.fuku@fuku.mcp_servers.fuku.default_tools_approval_mode="approve"`.
  The plugin manifest stays as it is, so an interactive Codex user still gets the approval prompt
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
- Claude Code is 2.1.319 or newer, and codex-cli is 0.166.0 or newer. The runner refuses an older host
- fuku is built: `make build`

Commands:

```sh
make build
make test:agents-e2e
make test:agents-e2e HOST=codex SCENARIO=S4 REPEATS=1
```

- `HOST` is `claude` or `codex`. Unset runs both
- `SCENARIO` is a scenario ID, or a comma-separated list. Unset runs all
- `REPEATS` is the number of runs per host and scenario. The default is 3
- `make` also reads `HOST` from the environment. Some shells export it as the machine name, and the target then exits 2 with "HOST must be one of claude, codex"
- in that case pass it explicitly: `make test:agents-e2e HOST=` runs both hosts

Cost and quota:

- every run is a real agent session on your own subscription
- a full sweep is 2 hosts × 15 scenarios × 3 repeats, so 90 runs
- Codex runs draw on your Codex plan's quota
- the scenario budgets cap one run at $0.75 to $1.50
- `tool_calls` leaves out the host's own skill reads, so both hosts are measured alike.
  That is one successful Claude Code `Skill` call of `fuku:fuku` and of `fuku:fuku-config`, or one Codex call that only reads each `SKILL.md`.
  A repeat read, a failed read or a Codex call that also runs another command counts
- an account limit marks the run as a host outage, stops that host and makes the sweep exit 1.
  A missing login does the same, and so does a host older than its minimum version, before its first run
- SIGTERM stops a sweep the way Ctrl-C does: the host group is killed, the auth copy and the temp project are removed, and the runner exits 143.
  A repeated SIGTERM is ignored, so it cannot cut the cleanup short.
  The status lines of the finished runs stay on stdout. No report file, table or guard check follows

Do not use Claude Code or Codex for other work while a sweep runs.
The guards compare the state before and after the sweep:

- `~/.codex/config.toml` and `~/.claude/settings.json` by content hash
- the paths under `~/.codex/plugins` whose name contains `fuku`
- every file, link and directory under `~/.claude/plugins` by relative path and content hash.
  A directory the guard cannot read is recorded as unreadable. Nothing is capped, so a large plugin tree makes the guard slower
- `~/.claude/plugins/store/`, `known_marketplaces.json` and `plugin-directory-cache-v2.json` at the top of `~/.claude/plugins` are left out.
  Claude Code writes the usage queue of its built-in plugins and refreshes the marketplace files during any session

A plugin install, a changed metadata value or an edited cached file from another session fails the sweep.
So does a marketplace update or a Claude Code plugin command during the sweep.

The offline self-checks run in `make test:agents-plugin` and spend no tokens.
The tests that need a built fuku skip there. Run them with `make build && FUKU_BIN=$PWD/cmd/fuku python3 -m unittest discover -s plugins/agents/tests -p 'test_e2e_*.py'`.

## The report

The sweep writes to a new temp folder outside the repository and prints its path.

- `report.json` holds, per host and scenario, the pass rate and each failed check with its reason
- it also holds the medians of tool calls, input tokens, output tokens, seconds and USD, the host version and the model
- each run has flags: `over_budget`, `timed_out`, `host_outage`, `credentials_read` and `login_rotated`
- the sweep lists the largest fuku tool outputs by bytes
- one file per run, `<host>-<scenario>-<n>.json`, holds the redacted commands and outputs. Raw host output is never written
- it also holds `answer`, the agent's final message, redacted and cut to 4000 characters. It is empty when the host gave none

Nothing in the report should be a secret. It still holds absolute paths and command output, so do not share it blindly.

Exit codes:

- `0`: every scenario passed every repeat on every host, and no guard failed
- `1`: a scenario fell below 100%, was not run, a host had an outage, no login or an old version, or a guard failed
- `2`: a usage error, a scenario file error or a missing fuku binary. No host starts

## Scenarios

The IDs follow the groups:

- Start: S1 starts the profile, S2 finds fuku already running, S3 starts the `core` profile
- Inspect: S4 lists the running services, S5 finds why `worker` stopped, S6 reads the command of `api`
- Stop: S7 restarts `api`, S8 stops `worker` and keeps `api`, S9 stops fuku
- Edit: S10 edits a service that does not watch, S11 edits a watching service
- Config: S12 applies a change under a detached fuku, S13 under a foreground fuku, S14 runs an experiment
- Limits: S15 asks to follow the api logs live, which no tool does

One JSON file per scenario in `scenarios/`, named by its ID. The keys are exactly `prompt`, `profile`, `setup`, `checks` and `budget`.

- `profile` must be a profile of `fixture/fuku.yaml`
- `setup` runs in order before the prompt:
  - `start_fuku`
  - `start_fuku_foreground`
  - `override:<template>` with a template from `fixture/overrides/`
  - `wait_service_exited:<service>`
  - `edit_service_command:<service>`
- the budgets of S8, S10, S11, S13 and S14 are estimates until a sweep measures them
- S8 stops one service and keeps the rest: the agent must stop `worker` with `service`, never `stop` fuku or kill the process
- S9 stops fuku because the user asks: `fuku_stopped` needs no fuku on the project socket and no process left of the fuku or the services the setup started
- S10 has no `start_fuku`: the prompt asks the agent to start fuku first, then edit `api/greeting.txt`, so it must restart `api` before `e2e.sh` passes.
  Its `greeting` override makes `api` copy the greeting to `served.txt` at start, so `e2e.sh` passes only against an `api` started after the edit.
  `project_e2e_passes` runs the fixture's `e2e.sh` in the temp project, not the project's copy, so an edited script does not count.
  `service_applied:api` needs a successful `service` call for `api`, so a `served.txt` the agent wrote by hand does not count
- S11 is S10 with a watching `api`, under a detached fuku: its `greeting-watch` override watches `greeting.txt` only, so the `served.txt` that `api` writes does not restart it again.
  The edit restarts `api` on its own. The agent must not call `service`. It waits in `status` for the new PID, then tests
- S13 is S12 under a fuku the developer runs in the foreground: `start_fuku_foreground` starts `fuku run <profile> --no-ui` as a child of the runner and waits until it serves every service.
  The runner keeps it alive for the run and ends it in the teardown with the other processes of the project.
  `status` reports `mode: foreground`, so the agent must not call `stop`, `run` or `service`. It tells the user the change needs a restart of their fuku
- S14 is an experiment under a detached fuku: the prompt asks for a temporary change to the `api` command, a look at its logs, then the config as it was.
  The agent must make the change in `fuku.override.yaml`, never in `fuku.yaml` or a config file of its own, restore the override byte for byte and apply both with `stop` then `run`.
  `run_detached_twice` proves two applies, one for the experiment and one for the restore.
  `override_restored` and `config_unchanged` also fail on a leftover `fuku.override.yaml.bak` or `fuku.yaml.bak`.
  `config_unchanged` also fails on a `fuku.yaml` written after the setup, even when its text is back to the fixture's
- S15 asks for something no tool does: `logs` never follows. The agent must change nothing in fuku and say which tool is missing.
  The checks prove that it changed nothing, in the processes and in `fuku.yaml` and the override, and made no workaround. Whether its answer names the missing tool is read in the report's `answer`
- S6 asks for the command of `api`, with the token in the override. The agent must read it through `config`, not from a config file.
  `config_read` needs a successful `config` call.
  `no_config_file` fails on a shell command, Read or Grep call that names `fuku.yaml` or the override, and on a recursive `grep` or `rg` of the project root whose globs do not exclude them.
  A command that lists only file names or counts, such as `ls`, `rg --files` or `grep -c`, passes.
  `rg` globs follow ripgrep's rules, so `**/fuku*` and `/fuku*` match `fuku.yaml`.
  The grader treats `grep --exclude` as overriding `--include`, as ugrep (the `grep` of Claude Code) does.
  BSD grep (the `grep` of Codex) lets the last matching rule win, so the grader can miss a read there but never fails a compliant run.
  One table of the options that take a value, in `checks.py`, decides which word is an option value, a listing flag or a path. A glob's value is never a path
- `checks` names state checks and command checks:
  - state: `fuku_running`, `fuku_stopped`, `services_running`, `same_fuku_pid`, `same_service_pids`, `fuku_pid_changed`, `served_by_built_fuku`, `service_pid_changed:<service>`, `other_service_pids_same:<service>`, `service_command_is:<service>`, `service_stopped:<service>`, `project_e2e_passes`, `override_restored`, `config_unchanged`
  - calls: `run_detached_once`, `run_detached_twice`, `status_read`, `log_read_with_marker`, `service_log_read:<service>`, `no_fuku_run`, `no_default_run`, `no_fuku_stop`, `no_kill`, `no_api_restart`, `service_applied:<service>`, `skill_loaded`, `no_config_argument`, `no_probe`, `config_read`, `no_config_file`
  - the call checks grade the fuku MCP calls: one successful `run` of the profile, two successful `run`s of the profile, a successful `status`, a `logs` result with the marker or a line of the service, no `run`, no `run` of `default`, no `stop`, no `service` call, a successful `service` call of any action that names the service, no fuku call with a `config` argument.
    `no_kill` reads the shell commands
  - `no_probe` runs on S1–S9 and S15, the tasks that only control fuku. It fails on a shell call that runs or probes the `fuku` CLI, such as `fuku --help`, `which fuku`, `command -v fuku`, `$(which fuku)`, `pgrep fuku` or `ps aux | grep fuku`, and on a script, run through an interpreter such as `python3`, through `sh` or `bash`, or by name such as `./poll.sh`.
    A file search such as `rg --files -g 'fuku*'` or `grep fuku`, a skill read and a grep of one config key pass. They find files and change nothing
  - `service_command_is:<service>` compares the service's process with the command `edit_service_command` wrote, or with the fixture's command when the setup wrote none
  - `skill_loaded` runs on S12, S13 and S14 only, where the config-edit rules decide the outcome. It means the `fuku-config` skill loaded. The short `fuku` skill triggers on every fuku task and is not graded.
    Claude Code loads `fuku-config` when its `Skill` call of `fuku:fuku-config` succeeds from the working-tree plugin.
    Codex loads it when one successful call runs a reader such as `cat`, `sed` or `nl` on the installed `skills/fuku-config/SKILL.md` and its output holds every line of the file. Listing or echoing the path, or a partial read, does not count
- `budget` holds `tool_calls`, `input_tokens`, `seconds` and `usd`. `usd` applies only where the host reports it

Every run also checks, whatever the scenario says:

- no shell call to `fuku` or `api.sh`, and no HTTP client against `/api/v1`. fuku is driven only through its MCP tools
- no whole-file read of a fuku config. A bounded single-key read such as `grep -n '^ *listen:' fuku.yaml` is allowed
- no kill of fuku or a service, and no removal of a fuku socket or lock.
  A `pkill` or `killall` pattern counts as a kill when it matches `fuku`, a service program such as `sleep 3601`, or a service dir. So `pkill sleep` fails
- no `config` argument of a fuku tool outside the temp project
- the API token never appears and no host login was read

An unknown key, setup name or check name is refused before any host starts.

## Changing it

- the minimum versions are Claude Code 2.1.319 and codex-cli 0.166.0, set as `MIN_VERSION` in `claude.py` and `codex.py`.
  They are the oldest versions a sweep verified the adapters on. A newer host runs.
  An older host, or one whose `--version` names no version, is refused before its first run, and the sweep exits 1.
  The report records the host version of each run in `host_version`
- an event the adapter cannot read fails the run as `unreadable`.
  When a host changes its event shape, record a new sanitized sample in `../tests/samples/` and update the parser
- the shell checks read shell text, not intent. They miss a command hidden in a script file, in `python -c` or in a command substitution such as `$(fuku run)`
- `claude-mcp.jsonl` and `codex-mcp.jsonl` in `../tests/samples/` are built from the host docs, not recorded.
  Replace them with sanitized recorded events after the first sweep, and check the `mcp_servers` entry of the Claude Code init event
- call checks compare sets and counts, never order. Runs differ in order
- keep every secret out of what the sweep writes: redact first, then cut
- keep this folder free of a `SKILL.md`. The structure check expects exactly the two skills `fuku` and `fuku-config` in the plugin
